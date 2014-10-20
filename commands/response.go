package commands

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

// ErrorType signfies a category of errors
type ErrorType uint

// ErrorTypes convey what category of error ocurred
const (
	ErrNormal ErrorType = iota // general errors
	ErrClient                  // error was caused by the client, (e.g. invalid CLI usage)
	// TODO: add more types of errors for better error-specific handling
)

// Error is a struct for marshalling errors
type Error struct {
	Message string
	Code    ErrorType
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d error: %s", e.Code, e.Message)
}

// EncodingType defines a supported encoding
type EncodingType string

// Supported EncodingType constants.
const (
	JSON = "json"
	XML  = "xml"
	// TODO: support more encoding types
)

// Marshaller is a function used by coding types.
// TODO this should just be a `coding.Codec`
type Marshaller func(v interface{}) ([]byte, error)

var marshallers = map[EncodingType]Marshaller{
	JSON: json.Marshal,
	XML:  xml.Marshal,
}

// Response is the result of a command request. Handlers write to the response,
// setting Error or Value. Response is returned to the client.
type Response struct {
	req   *Request
	Error *Error
	Value interface{}
}

// SetError updates the response Error.
func (r *Response) SetError(err error, code ErrorType) {
	r.Error = &Error{Message: err.Error(), Code: code}
}

// Marshal marshals out the response into a buffer. It uses the EncodingType
// on the Request to chose a Marshaller (Codec).
func (r *Response) Marshal() ([]byte, error) {
	if r.Error == nil && r.Value == nil {
		return nil, fmt.Errorf("No error or value set, there is nothing to marshal")
	}

	enc, ok := r.req.Option(EncShort)
	if !ok || enc.(string) == "" {
		return nil, fmt.Errorf("No encoding type was specified")
	}
	encType := EncodingType(strings.ToLower(enc.(string)))

	marshaller, ok := marshallers[encType]
	if !ok {
		return nil, fmt.Errorf("No marshaller found for encoding type '%s'", enc)
	}

	if r.Error != nil {
		return marshaller(r.Error)
	}
	return marshaller(r.Value)
}
