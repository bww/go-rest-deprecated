package rest

import (
  "fmt"
  "net/http"
)

/**
 * A request error
 */
type Error struct {
  Status    int
  Headers   map[string]string
  Cause     error
  Detail    interface{}
}

/**
 * Create a status error
 */
func NewError(s int, e error) *Error {
  return &Error{s, nil, e, nil}
}

/**
 * Create a status error
 */
func NewErrorf(s int, f string, a ...interface{}) *Error {
  return &Error{s, nil, basicError{s, fmt.Sprintf(f, a...)}, nil}
}

/**
 * Set headers
 */
func (e *Error) SetHeaders(h map[string]string) *Error {
  e.Headers = h
  return e
}

/**
 * Set detail
 */
func (e *Error) SetDetail(d interface{}) *Error {
  e.Detail = d
  return e
}

/**
 * Obtain the error message
 */
func (e Error) Error() string {
  if c := e.Cause; c != nil {
    return c.Error()
  }else{
    return http.StatusText(e.Status)
  }
}

/**
 * A simple error
 */
type basicError struct {
  Status    int     `json:"status"`
  Message   string  `json:"message"`
}

/**
 * It's an error, folks
 */
func (e basicError) Error() string {
  return e.Message
}

// An error that has supplemental details
type ErrorDetail interface {
  ErrorDetail()(interface{})
}

// An error that represents a field
type FieldError interface {
  ErrorField()(string)
  ErrorMessage()(string)
}
