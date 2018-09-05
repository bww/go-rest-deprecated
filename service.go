package rest

import (
  "io"
  "os"
  "fmt"
  "time"
  "regexp"
  "reflect"
  "strings"
  "strconv"
  "net/http"
  "encoding/json"
)

import (
  "golang.org/x/net/html"
  "github.com/gorilla/mux"
  "github.com/bww/go-alert"
  "github.com/bww/go-util/text"
)

// Internal service options
type serviceOptions uint32
const (
  serviceOptionNone = serviceOptions(0)
)

/**
 * Service config
 */
type Config struct {
  Name          string
  Instance      string
  Hostname      string
  UserAgent     string
  ReadTimeout   time.Duration
  WriteTimeout  time.Duration
  IdleTimeout   time.Duration
  Endpoint      string
  TraceRegexps  []*regexp.Regexp
  EntityHandler EntityHandler
  Debug         bool
}

/**
 * A REST service
 */
type Service struct {
  name          string
  instance      string
  hostname      string
  userAgent     string
  port          string
  router        *mux.Router
  pipeline      Pipeline
  traceRequests map[string]*regexp.Regexp
  entityHandler EntityHandler
  debug         bool
  options       serviceOptions
  readTimeout   time.Duration
  writeTimeout  time.Duration
  idleTimeout   time.Duration
  suppress      map[string]struct{}
}

/**
 * Create a new service
 */
func NewService(c Config) *Service {
  
  s := &Service{}
  s.instance = c.Instance
  s.hostname = c.Hostname
  s.userAgent = c.UserAgent
  s.port = c.Endpoint
  s.router = mux.NewRouter()
  s.entityHandler = c.EntityHandler
  s.readTimeout = c.ReadTimeout
  s.writeTimeout = c.WriteTimeout
  s.idleTimeout = c.IdleTimeout
  
  if c.Name == "" {
    s.name = "service"
  }else{
    s.name = c.Name
  }
  
  if c.Debug || os.Getenv("GOREST_DEBUG") == "true" {
    s.debug = true
  }
  
  if c.TraceRegexps != nil {
    if s.traceRequests == nil {
      s.traceRequests = make(map[string]*regexp.Regexp)
    }
    for _, e := range c.TraceRegexps {
      s.traceRequests[e.String()] = e
    }
  }
  if t := os.Getenv("GOREST_TRACE"); t != "" {
    if s.traceRequests == nil {
      s.traceRequests = make(map[string]*regexp.Regexp)
    }
    for _, e := range strings.Split(t, ";") {
      s.traceRequests[e] = regexp.MustCompile(e)
    }
  }
  if s.debug {
    for k, _ := range s.traceRequests {
      fmt.Println("rest: trace:", k)
    }
  }
  
  s.suppress = make(map[string]struct{})
  if v := os.Getenv("GOREST_TRACE_SUPPRESS_HEADERS"); v != "" {
    if !strings.EqualFold(v, "none") {
      for _, e := range strings.Split(v, ",") {
        s.suppress[strings.ToLower(e)] = struct{}{}
      }
    }
  }else{
    s.suppress["authorization"] = struct{}{}
  }
  
  return s
}

/**
 * Create a context
 */
func (s *Service) Context() *Context {
  return newContext(s, s.router)
}

/**
 * Obtain the root router, if you must.
 */
func (s *Service) Router() *mux.Router {
  return s.router
}

/**
 * Create a subrouter that can be configured for specialized use
 */
func (s *Service) Subrouter(p string) *mux.Router {
  return s.router.PathPrefix(p).Subrouter()
}

/**
 * Create a context scoped under a base path
 */
func (s *Service) ContextWithBasePath(p string) *Context {
  return newContext(s, s.router.PathPrefix(p).Subrouter())
}

/**
 * Attach a handler to the service pipeline
 */
func (s *Service) Use(h ...Handler) {
  if h != nil {
    for _, e := range h {
      s.pipeline = s.pipeline.Add(e)
    }
  }
}

/**
 * Run the service (this blocks forever)
 */
func (s *Service) Run() error {
  s.pipeline = s.pipeline.Add(HandlerFunc(s.routeRequest))
  
  server := &http.Server{
    Addr: s.port,
    Handler: s,
    ReadTimeout: s.readTimeout,
    WriteTimeout: s.writeTimeout,
    IdleTimeout: s.idleTimeout,
  }
  
  alt.Debugf("%s: Listening on %v", s.name, s.port)
  return server.ListenAndServe()
}

/**
 * Display all routes in the service
 */
func (s *Service) DumpRoutes(w io.Writer) error {
  return s.router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
    p, err := route.GetPathTemplate()
    if err != nil {
      return err
    }
    fmt.Fprintf(w, "  %v", p)
    fmt.Fprintln(w)
    return nil
  })
  return nil
}

/**
 * Request handler
 */
func (s *Service) ServeHTTP(rsp http.ResponseWriter, req *http.Request) {
  wreq := newRequest(req)
  res, err := s.pipeline.Next(rsp, wreq)
  if res != nil || err != nil {
    s.sendResponse(rsp, wreq, res, err)
  }
}

/**
 * Default (routing) request handler; this is a bit weird, the context will
 * handle the result, so we return nothing from here
 */
func (s *Service) routeRequest(rsp http.ResponseWriter, req *Request, pln Pipeline) (interface{}, error) {
  s.router.ServeHTTP(rsp, req.Request)
  return nil, nil
}

/**
 * Send a result
 */
func (s *Service) sendResponse(rsp http.ResponseWriter, req *Request, res interface{}, err error) {
  rsp.Header().Set("X-Request-Id", req.Id)
  if err == nil {
    s.sendSuccess(rsp, req, res)
  }else{
    s.sendError(rsp, req, err)
  }
}

/**
 * Send success
 */
func (s *Service) sendSuccess(rsp http.ResponseWriter, req *Request, res interface{}) {
  var r int
  var e interface{}
  var h map[string]string
  
  switch v := res.(type) {
    case *Response:
      r = v.StatusCode
      e = v.Entity
      h = v.Headers
    default:
      r = http.StatusOK
      e = res
  }
  
  s.sendEntity(rsp, req, r, h, e)
}

/**
 * Respond with an error
 */
func (s *Service) sendError(rsp http.ResponseWriter, req *Request, err error) {
  var m string
  var r int
  var c error
  var h map[string]string
  
  switch v := err.(type) {
    case *Error:
      r = v.Status
      h = v.Headers
      c = v.Cause
      m = fmt.Sprintf("%s: [%v] %v", s.name, req.Id, c)
      if d := formatDetail(c); d != "" {
        m += "\n"+ d
      }
    default:
      r = http.StatusInternalServerError
      c = basicError{http.StatusInternalServerError, err.Error()}
      m = fmt.Sprintf("%s: [%v] %v", s.name, req.Id, err)
  }
  
  // propagate non-success, non-client errors; just log others
  if r < 200 || r >= 500 {
    alt.Error(m, nil, nil)
  }else{
    alt.Debug(m)
  }
  if req.Accepts("text/html") {
    s.sendEntity(rsp, req, r, h, htmlError(r, h, c))
  }else{
    s.sendEntity(rsp, req, r, h, c)
  }
}

/**
 * Respond with an entity
 */
func (s *Service) sendEntity(rsp http.ResponseWriter, req *Request, status int, headers map[string]string, content interface{}) {
  
  if headers != nil {
    for k, v := range headers {
      rsp.Header().Add(k, v)
    }
  }
  if ua := s.userAgent; ua != "" {
    rsp.Header().Add("User-Agent", ua)
  }
  
  var err error
  if s.entityHandler != nil {
    err = s.entityHandler(rsp, req, status, content)
  }else{
    err = DefaultEntityHandler(rsp, req, status, content)
  }
  if err != nil {
    alt.Errorf("%s: %v", s.name, err)
    return
  }
  
}

/**
 * Produce a HTML error entity
 */
func htmlError(status int, headers map[string]string, content error) Entity {
  
  e := html.EscapeString(content.Error())
  
  m := `<html><body>`
  m += `<h1>`+ fmt.Sprintf("%v %v", status, http.StatusText(status)) +`</h1>`
  m += `<p>`+ e +`</p>`
  
  var detail interface{}
  if v, ok := content.(ErrorDetail); ok {
    detail = v.ErrorDetail()
  }
  
  v := reflect.ValueOf(detail)
  if v.IsValid() && !v.IsNil() {
    if v.Kind() == reflect.Map {
      m += `<table>`
      for _, e := range v.MapKeys() {
        x := v.MapIndex(e)
        m += `<tr>`
        m += fmt.Sprintf(`  <td><strong>%s</strong></td>`, html.EscapeString(fmt.Sprintf("%v", e.Interface())))
        m += fmt.Sprintf(`  <td>%s</td>`, html.EscapeString(fmt.Sprintf("%v", x.Interface())))
        m += `</tr>`
      }
      m += `</table>`
    }else if v.Kind() == reflect.Slice {
      m += `<table>`
      for i := 0; i < v.Len(); i++ {
        x := v.Index(i).Interface()
        if f, ok := x.(FieldError); ok {
          m += `<tr>`
          m += fmt.Sprintf(`  <td><strong><code>%v</code></strong></td>`, html.EscapeString(f.ErrorField()))
          m += fmt.Sprintf(`  <td>%v</td>`, html.EscapeString(f.ErrorMessage()))
          m += `</tr>`
        }else{
          m += `<tr>`
          m += fmt.Sprintf(`  <td>%v</td>`, x)
          m += `</tr>`
        }
      }
      m += `</table>`
    }
  }
  
  m += `</body></html>`
  
  return NewBytesEntity("text/html", []byte(m))
}

func formatDetail(c interface{}) string {
  var s string
  var detail interface{}
  if v, ok := c.(ErrorDetail); ok {
    detail = v.ErrorDetail()
    v := reflect.ValueOf(detail)
    if v.IsValid() && !v.IsNil() {
      if v.Kind() == reflect.Map {
        for _, e := range v.MapKeys() {
          x := v.MapIndex(e)
          s += fmt.Sprintf("  - %s: %s\n", text.Stringer(e), formatValue(x))
        }
      }else if v.Kind() == reflect.Slice {
        for i := 0; i < v.Len(); i++ {
          x := v.Index(i)
          s += fmt.Sprintf("  - %s\n", formatValue(x))
        }
      }
    }
  }
  return s
}

func formatValue(v reflect.Value) string {
  if v.IsValid() && !v.IsNil() {
    v := v.Interface()
    switch c := v.(type) {
      case string:
        return c
      case bool:
        return strconv.FormatBool(c)
      case rune: // is really int32
        return string(c)
      case int:
        return strconv.FormatInt(int64(c), 10)
      case int8:
        return strconv.FormatInt(int64(c), 10)
      case int16:
        return strconv.FormatInt(int64(c), 10)
      case int64:
        return strconv.FormatInt(int64(c), 10)
      case uint8:
        return strconv.FormatUint(uint64(c), 10)
      case uint16:
        return strconv.FormatUint(uint64(c), 10)
      case uint32:
        return strconv.FormatUint(uint64(c), 10)
      case uint64:
        return strconv.FormatUint(uint64(c), 10)
      default:
        d, _ := json.MarshalIndent(v, "", "  ")
        return text.IndentWithOptions(string(d), "    ", 0)
    }
  }
  return ""
}
