package commands

import (
	"io"

	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
)

// ErrorType signfies a category of errors
type ErrorType uint

// EncodingType defines a supported encoding
type EncodingType string

// Supported EncodingType constants.
const (
	JSON     = "json"
	XML      = "xml"
	Protobuf = "protobuf"
	Text     = "text"
	// TODO: support more encoding types
)

// Response is the result of a command request. Handlers write to the response,
// setting Error or Value. Response is returned to the client.
type Response interface {
	Request() Request

	// Set/Return the response Error
	SetError(err error, code cmdkit.ErrorType)
	Error() *cmdkit.Error

	// Sets/Returns the response value
	SetOutput(interface{})
	Output() interface{}

	// Sets/Returns the length of the output
	SetLength(uint64)
	Length() uint64

	// underlying http connections need to be cleaned up, this is for that
	Close() error
	SetCloser(io.Closer)

	// Marshal marshals out the response into a buffer. It uses the EncodingType
	// on the Request to chose a Marshaler (Codec).
	Marshal() (io.Reader, error)

	// Gets a io.Reader that reads the marshalled output
	Reader() (io.Reader, error)

	// Gets Stdout and Stderr, for writing to console without using SetOutput
	Stdout() io.Writer
	Stderr() io.Writer
}
