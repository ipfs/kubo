package commands

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/jbenet/go-ipfs/config"
	"github.com/jbenet/go-ipfs/core"
)

type optMap map[string]interface{}

type Context struct {
	ConfigRoot string
	Config     *config.Config
	Node       *core.IpfsNode
}

// Request represents a call to a command from a consumer
type Request interface {
	Path() []string
	Option(name string) (interface{}, bool)
	Options() map[string]interface{}
	SetOption(name string, val interface{})
	Arguments() []interface{} // TODO: make argument value type instead of using interface{}
	Context() *Context
	SetContext(Context)
	Command() *Command

	ConvertOptions(options map[string]Option) error
}

type request struct {
	path      []string
	options   optMap
	arguments []interface{}
	cmd       *Command
	ctx       Context
}

// Path returns the command path of this request
func (r *request) Path() []string {
	return r.path
}

// Option returns the value of the option for given name.
func (r *request) Option(name string) (interface{}, bool) {
	val, err := r.options[name]
	return val, err
}

// Options returns a copy of the option map
func (r *request) Options() map[string]interface{} {
	output := make(optMap)
	for k, v := range r.options {
		output[k] = v
	}
	return output
}

// SetOption sets the value of the option for given name.
func (r *request) SetOption(name string, val interface{}) {
	r.options[name] = val
}

// Arguments returns the arguments slice
func (r *request) Arguments() []interface{} {
	return r.arguments
}

func (r *request) Context() *Context {
	return &r.ctx
}

func (r *request) SetContext(ctx Context) {
	r.ctx = ctx
}

func (r *request) Command() *Command {
	return r.cmd
}

type converter func(string) (interface{}, error)

var converters = map[reflect.Kind]converter{
	Bool: func(v string) (interface{}, error) {
		if v == "" {
			return true, nil
		}
		return strconv.ParseBool(v)
	},
	Int: func(v string) (interface{}, error) {
		return strconv.ParseInt(v, 0, 32)
	},
	Uint: func(v string) (interface{}, error) {
		return strconv.ParseInt(v, 0, 32)
	},
	Float: func(v string) (interface{}, error) {
		return strconv.ParseFloat(v, 64)
	},
}

func (r *request) ConvertOptions(options map[string]Option) error {
	converted := make(map[string]interface{})

	for k, v := range r.options {
		opt, ok := options[k]
		if !ok {
			continue
		}

		kind := reflect.TypeOf(v).Kind()
		var value interface{}

		if kind != opt.Type {
			if kind == String {
				convert := converters[opt.Type]
				str, ok := v.(string)
				if !ok {
					return errors.New("cast error")
				}
				val, err := convert(str)
				if err != nil {
					return fmt.Errorf("Could not convert string value '%s' to type '%s'",
						v, opt.Type.String())
				}
				value = val

			} else {
				return fmt.Errorf("Option '%s' should be type '%s', but got type '%s'",
					k, opt.Type.String(), kind.String())
			}
		} else {
			value = v
		}

		for _, name := range opt.Names {
			if _, ok := r.options[name]; name != k && ok {
				return fmt.Errorf("Duplicate command options were provided ('%s' and '%s')",
					k, name)
			}

			converted[name] = value
		}
	}

	r.options = converted
	return nil
}

// NewEmptyRequest initializes an empty request
func NewEmptyRequest() Request {
	return NewRequest(nil, nil, nil, nil)
}

// NewRequest returns a request initialized with given arguments
func NewRequest(path []string, opts optMap, args []interface{}, cmd *Command) Request {
	if path == nil {
		path = make([]string, 0)
	}
	if opts == nil {
		opts = make(map[string]interface{})
	}
	if args == nil {
		args = make([]interface{}, 0)
	}
	return &request{path, opts, args, cmd, Context{}}
}
