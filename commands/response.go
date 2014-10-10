package commands

type ErrorType uint
const (
  Normal ErrorType = iota // general errors
  Client // error was caused by the client, (e.g. invalid CLI usage)
  // TODO: add more types of errors for better error-specific handling
)

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

/*func (r *Response) Encode() ([]byte, error) {

}*/
