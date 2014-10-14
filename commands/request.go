package commands

// Request represents a call to a command from a consumer
type Request struct {
	path			[]string
	options   map[string]interface{}
	arguments []string
}

func (r *Request) Path() []string {
	return r.path
}

func (r *Request) Option(name string) interface{} {
	return r.options[name]
}

func (r *Request) SetOption(option Option, value interface{}) {
	// saves the option value in the map, indexed by each name
	// (so commands can retrieve it using any of the names)
	for _, name := range option.Names {
		r.options[name] = value
	}
}

func (r *Request) Arguments() []string {
	return r.arguments
}

func NewRequest() *Request {
	return &Request{
		make([]string, 0),
		make(map[string]interface{}),
		make([]string, 0),
	}
}
