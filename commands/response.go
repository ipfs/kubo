package commands

import (
  "fmt"
  "strings"
  "encoding/json"
  "encoding/xml"
)

type ErrorType uint
const (
  Normal ErrorType = iota // general errors
  Client // error was caused by the client, (e.g. invalid CLI usage)
  // TODO: add more types of errors for better error-specific handling
)

// Error is a struct for marshalling errors
type Error struct {
  message string
  code ErrorType
}

type EncodingType string
const (
  Json = "json"
  Xml = "xml"
  // TODO: support more encoding types
)

type Marshaller func(v interface{})([]byte, error)
var marshallers = map[EncodingType]Marshaller{
  Json: json.Marshal,
  Xml: xml.Marshal,
}

type Response struct {
  req *Request
  Error error
  ErrorType ErrorType
  Value interface{}
}

func (r *Response) SetError(err error, errType ErrorType) {
  r.Error = err
  r.ErrorType = errType
}

func (r *Response) FormatError() Error {
  return Error{ r.Error.Error(), r.ErrorType }
}

func (r *Response) Marshal() ([]byte, error) {
  enc := r.req.Option("enc")
  if enc == nil {
    return nil, fmt.Errorf("No encoding type was specified")
  }
  encType := EncodingType(strings.ToLower(enc.(string)))

  marshaller, ok := marshallers[encType]
  if !ok {
    return nil, fmt.Errorf("No marshaller found for encoding type '%s'", enc)
  }

  return marshaller(r.Value)
}
