package trace

import (
  "fmt"
  "time"
)

// A trace
type Trace interface {
  Timestamp()(time.Time)
  Context()(interface{})
  Message()(string)
  Error()(error)
}

// A non-error message trace
type messageTrace struct {
  when    time.Time
  message string
  context interface{}
}

func NewMessage(m string) Trace {
  return &messageTrace{time.Now(), m, nil}
}

func NewMessagef(f string, a ...interface{}) Trace {
  return &messageTrace{time.Now(), fmt.Sprintf(f, a...), nil}
}

func (t messageTrace) Timestamp() time.Time {
  return t.when
}

func (t messageTrace) Message() string {
  return t.message
}

func (t messageTrace) Error() error {
  return nil
}

func (t messageTrace) Context() interface{} {
  return t.context
}

// An error trace
type errorTrace struct {
  when    time.Time
  err     error
  context interface{}
}

func NewError(e error) Trace {
  return &errorTrace{time.Now(), e, nil}
}

func NewErrorf(f string, a ...interface{}) Trace {
  return &errorTrace{time.Now(), fmt.Errorf(f, a...), nil}
}

func (t errorTrace) Timestamp() time.Time {
  return t.when
}

func (t errorTrace) Error() error {
  return t.err
}

func (t errorTrace) Message() string {
  return ""
}

func (t errorTrace) Context() interface{} {
  return t.context
}

