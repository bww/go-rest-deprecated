package rest

import (
  "fmt"
  "time"
  "bytes"
  "strings"
  "io/ioutil"
  "net/http"
  "net/http/httptest"
)

import (
  "github.com/gorilla/mux"
  "github.com/bww/go-alert"
  "github.com/bww/go-util/text"
)

/**
 * A service context
 */
type Context struct {
  service   *Service
  router    *mux.Router
  pipeline  Pipeline
}

/**
 * Create a context
 */
func newContext(s *Service, r *mux.Router) *Context {
  return &Context{s, r, nil}
}

/**
 * Attach a handler to the context pipeline
 */
func (c *Context) Use(h ...Handler) {
  if h != nil {
    for _, e := range h {
      c.pipeline = c.pipeline.Add(e)
    }
  }
}

/**
 * Create a route
 */
func (c *Context) HandleFunc(u string, f func(http.ResponseWriter, *Request, Pipeline)(interface{}, error), a ...Attrs) *mux.Route {
  return c.Handle(u, c.pipeline.Add(HandlerFunc(f)), a...)
}

/**
 * Create a route
 */
func (c *Context) Handle(u string, h Handler, a ...Attrs) *mux.Route {
  attr := mergeAttrs(a...)
  return c.router.HandleFunc(u, func(rsp http.ResponseWriter, req *http.Request){
    c.handle(rsp, newRequestWithAttributes(req, attr), h)
  })
}

/**
 * Handle a request
 */
func (c *Context) handle(rsp http.ResponseWriter, req *Request, h Handler) {
  start := time.Now()
  
  // deal with proxies
  if r := req.Header.Get("X-Forwarded-For"); r != "" {
    req.RemoteAddr = r
  }else if r = req.Header.Get("X-Origin-IP"); r != "" {
    req.RemoteAddr = r
  }
  
  // where is this request endpoint, including parameters
  var where string
  if q := req.URL.Query(); q != nil && len(q) > 0 {
    where = fmt.Sprintf("%s?%v", req.URL.Path, q.Encode())
  }else{
    where = req.URL.Path
  }
  
  // determine if we need to trace the request
  trace := false
  if c.service.traceRequests != nil && len(c.service.traceRequests) > 0 {
    for _, e := range c.service.traceRequests {
      if e.MatchString(req.URL.Path) {
        alt.Debugf("%s: [%s] (trace:%v) %s %s ", c.service.name, req.RemoteAddr, e, req.Method, where)
        var reqdata string
        
        if req.Header != nil {
          for k, v := range req.Header {
            if _, ok := c.service.suppress[strings.ToLower(k)]; ok {
              reqdata += fmt.Sprintf("%v: <%v suppressed>\n", k, len(v))
            }else{
              reqdata += fmt.Sprintf("%v: %v\n", k, v)
            }
          }
        }
        
        if req.Body != nil {
          data, err := ioutil.ReadAll(req.Body)
          if err != nil {
            c.service.sendResponse(rsp, req, nil, NewError(http.StatusInternalServerError, err))
            return 
          }
          reqdata += "\n"
          if data != nil && len(data) > 0 {
            reqdata += string(data) +"\n"
          }
          req.Body = ioutil.NopCloser(bytes.NewBuffer(data))
        }
        
        fmt.Println(text.Indent(reqdata, "> "))
        fmt.Println("-")
        trace = true
        break
      }
    }
  }
  
  // handle the request itself and finalize if needed
  res, err := h.ServeRequest(rsp, req, nil)
  if (req.flags & reqFlagFinalized) != reqFlagFinalized {
    c.service.sendResponse(rsp, req, res, err)
    alt.Debugf("%s: [%v] (%v) %s %s", c.service.name, req.Id, time.Since(start), req.Method, where)
    if trace { // check for a trace and output the response
      recorder := httptest.NewRecorder()
      c.service.sendResponse(recorder, req, res, err)
      var rspdata string
      
      rspdata += fmt.Sprintf("HTTP/1.1 %v %v %s\n", recorder.Code, http.StatusText(recorder.Code), http.StatusText(recorder.Code))
      if recorder.HeaderMap != nil {
        for k, v := range recorder.HeaderMap {
          rspdata += fmt.Sprintf("%v: %v\n", k, v)
        }
      }
      
      rspdata += "\n"
      if b := recorder.Body; b != nil {
        rspdata += string(b.Bytes()) +"\n"
      }
      
      fmt.Println(text.Indent(rspdata, "< "))
      fmt.Println("#")
    }
  }
  
}

/**
 * Create a subrouter that can be configured for specialized use
 */
func (c *Context) Subrouter(p string) *mux.Router {
  return c.router.PathPrefix(p).Subrouter()
}
