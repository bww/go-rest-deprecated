package httputil

import (
  "io/ioutil"
  "net/http"
  "encoding/json"
)

import (
  "github.com/bww/go-rest"
)

/**
 * Read and return the request entity
 */
func RequestEntity(req *rest.Request) ([]byte, error) {
  
  if req.Body == nil {
    return nil, rest.NewErrorf(http.StatusBadRequest, "An entity is expected but the request has no body")
  }
  
  data, err := ioutil.ReadAll(req.Body)
  if err != nil {
    return nil, rest.NewErrorf(http.StatusBadRequest, "Could not read request entity: %v", err)
  }
  
  return data, nil
}

/**
 * Unmarshal a request entity. The entity is assumed to be JSON.
 */
func UnmarshalRequestEntity(req *rest.Request, entity interface{}) error {
  
  data, err := RequestEntity(req)
  if err != nil {
    return err
  }
  
  err = json.Unmarshal(data, entity)
  if err != nil {
    return rest.NewErrorf(http.StatusBadRequest, "Could not unmarshal request entity: %v", err)
  }
  
  return nil
}

/**
 * Returns a copy of the provided *http.Request. The clone is a shallow
 * copy of the struct and its Header map.
 */
func CopyRequest(r *http.Request) *http.Request {
  
  // shallow copy of the struct
  d := new(http.Request)
  *d = *r
  
  // deep copy of the Header
  d.Header = make(http.Header)
  for k, s := range r.Header {
    d.Header[k] = s
  }
  
  return d
}
