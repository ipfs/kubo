package commands

// Request represents a call to a command from a consumer
type Request struct {
  options map[string]interface{}
}

/*func (r *Request) Option(name string) interface{} {

}

func (r *Request) Arguments() interface{} {

}*/

func NewRequest() *Request {
  return &Request{
    make(map[string]interface{}),
  }
}
