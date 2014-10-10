package commands

// Request represents a call to a command from a consumer
type Request struct {
  options map[string]interface{}
  arguments []string
}

func (r *Request) Option(name string) interface{} {
  return r.options[name]
}

func (r *Request) Arguments() []string {
  return r.arguments
}

func NewRequest() *Request {
  return &Request{
    make(map[string]interface{}),
    make([]string, 0),
  }
}
