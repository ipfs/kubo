package commands

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

/*func (r *Response) Encode() ([]byte, error) {

}*/
