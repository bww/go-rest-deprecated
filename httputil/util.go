package httputil

import (
  "strings"
  "net/http"
  "io/ioutil"
  "encoding/json"
)

import (
  "github.com/bww/go-rest"
  "github.com/gorilla/schema"
)

var formDecoder *schema.Decoder
func init() {
  formDecoder = schema.NewDecoder()
  formDecoder.IgnoreUnknownKeys(true)
}

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

func UnmarshalRequestEntity(req *rest.Request, entity interface{}) error {
  switch strings.ToLower(req.Header.Get("Content-Type")) {
    case "application/x-www-form-urlencoded", "multipart/form-data":
      err := req.ParseForm()
      if err != nil {
        return rest.NewErrorf(http.StatusBadRequest, "Could not parse form: %v", err)
      }
      err = formDecoder.Decode(entity, req.PostForm)
      if err != nil {
        return rest.NewErrorf(http.StatusBadRequest, "Could not unmarshal request entity: %v", err)
      }
      
    case "application/json": fallthrough
    default:
      data, err := RequestEntity(req)
      if err != nil {
        return err
      }
      err = json.Unmarshal(data, entity)
      if err != nil {
        return rest.NewErrorf(http.StatusBadRequest, "Could not unmarshal request entity: %v", err)
      }
  }
  return nil
}

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
