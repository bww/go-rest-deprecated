package rest

import (
  "io"
  "fmt"
  "bytes"
  "net/http"
  "encoding/json"
)

/**
 * A response
 */
type Response struct {
  StatusCode  int
  Headers     map[string]string
  Entity      interface{}
}

/**
 * Create an entity context wrapper
 */
func NewResponse(r int, h map[string]string, e interface{}) *Response {
  return &Response{r, h, e}
}

/**
 * Create a redirect response
 */
func NewRedirect(loc string) *Response {
  return &Response{http.StatusFound, map[string]string{"Location": loc}, nil}
}

/**
 * Set a header value
 */
func (r *Response) Header(k, v string) *Response {
  if r.Headers == nil {
    r.Headers = make(map[string]string)
  }
  r.Headers[k] = v
  return r
}

/**
 * An entity
 */
type Entity interface {
  io.Reader
  ContentType()(string)
}

/**
 * An entity that wraps a reader
 */
type readerEntity struct {
  io.Reader
  contentType string
}

/**
 * Create a reader entity
 */
func NewReaderEntity(t string, r io.Reader) Entity {
  return &readerEntity{r, t}
}

/**
 * Content type
 */
func (e readerEntity) ContentType() string {
  return e.contentType
}

/**
 * A simple entity
 */
type BytesEntity struct {
  *bytes.Buffer
  contentType string
}

/**
 * Create a bytes entity
 */
func NewBytesEntity(t string, b []byte) *BytesEntity {
  return &BytesEntity{bytes.NewBuffer(b), t}
}

/**
 * Content type
 */
func (e BytesEntity) ContentType() string {
  return e.contentType
}

/**
 * An entity handler
 */
type EntityHandler func(http.ResponseWriter, *Request, int, interface{})(error)

/**
 * The default entity handler
 */
func DefaultEntityHandler(rsp http.ResponseWriter, req *Request, status int, content interface{}) error {
  switch e := content.(type) {
    
    case nil:
      rsp.WriteHeader(status)
      
    case Entity:
      rsp.Header().Add("Content-Type", e.ContentType())
      rsp.WriteHeader(status)
      
      n, err := io.Copy(rsp, e)
      if err != nil {
        return fmt.Errorf("Could not write entity: %v\n> In response to: %v %v\nEntity: %d bytes written", err, req.Method, req.URL, n)
      }
      
    case json.RawMessage:
      rsp.Header().Add("Content-Type", "application/json")
      rsp.WriteHeader(status)
      
      _, err := rsp.Write([]byte(e))
      if err != nil {
        return fmt.Errorf("Could not write entity: %v\n> In response to: %v %v\nEntity: %d bytes", err, req.Method, req.URL, len(e))
      }
      
    default:
      data, err := json.Marshal(content)
      if err != nil {
        rsp.WriteHeader(http.StatusInternalServerError) // don't return OK
        return fmt.Errorf("Could not marshal entity: %v\n> In response to: %v %v", err, req.Method, req.URL)
      }
      
      rsp.Header().Add("Content-Type", "application/json")
      rsp.WriteHeader(status)
      
      _, err = rsp.Write(data)
      if err != nil {
        return fmt.Errorf("Could not write entity: %v\n> In response to: %v %v\nEntity: %d bytes", err, req.Method, req.URL, len(data))
      }
      
  }
  return nil
}
