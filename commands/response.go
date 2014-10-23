package commands

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
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

func (e Error) Error() string {
	return e.Message
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
type Response interface {
	io.Reader

	Request() Request

	// Set/Return the response Error
	SetError(err error, code ErrorType)
	Error() *Error

	// Sets/Returns the response value
	SetValue(interface{})
	Value() interface{}

	// Marshal marshals out the response into a buffer. It uses the EncodingType
	// on the Request to chose a Marshaller (Codec).
	Marshal() ([]byte, error)
}

type response struct {
	req   Request
	err   *Error
	value interface{}
	out   io.Reader
}

func (r *response) Request() Request {
	return r.req
}

func (r *response) Value() interface{} {
	return r.value
}

func (r *response) SetValue(v interface{}) {
	r.value = v
}

func (r *response) Error() *Error {
	return r.err
}

func (r *response) SetError(err error, code ErrorType) {
	r.err = &Error{Message: err.Error(), Code: code}
}

func (r *response) Marshal() ([]byte, error) {
	if r.err == nil && r.value == nil {
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

	if r.err != nil {
		return marshaller(r.err)
	}
	return marshaller(r.value)
}

func (r *response) Read(p []byte) (int, error) {
	// if command set value to a io.Reader, set that as our output stream
	if r.out == nil {
		if out, ok := r.value.(io.Reader); ok {
			r.out = out
		}
	}

	// if there is an output stream set, read from it
	if r.out != nil {
		return r.out.Read(p)
	}

	// no stream set, so marshal the error or value
	output, err := r.Marshal()
	if err != nil {
		return 0, err
	}

	// then create a Reader from the marshalled data, and use it as our output stream
	r.out = bytes.NewReader(output)
	return r.out.Read(p)
}

// NewResponse returns a response to match given Request
func NewResponse(req Request) Response {
	return &response{req: req}
}
