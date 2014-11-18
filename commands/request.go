package commands

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/jbenet/go-ipfs/config"
	"github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"
)

type optMap map[string]interface{}

type Context struct {
	Online     bool
	ConfigRoot string

	config     *config.Config
	LoadConfig func(path string) (*config.Config, error)

	node          *core.IpfsNode
	ConstructNode func() (*core.IpfsNode, error)
}

// GetConfig returns the config of the current Command exection
// context. It may load it with the providied function.
func (c *Context) GetConfig() (*config.Config, error) {
	var err error
	if c.config == nil {
		if c.LoadConfig == nil {
			return nil, errors.New("nil LoadConfig function")
		}
		c.config, err = c.LoadConfig(c.ConfigRoot)
	}
	return c.config, err
}

// GetNode returns the node of the current Command exection
// context. It may construct it with the providied function.
func (c *Context) GetNode() (*core.IpfsNode, error) {
	var err error
	if c.node == nil {
		if c.ConstructNode == nil {
			return nil, errors.New("nil ConstructNode function")
		}
		c.node, err = c.ConstructNode()
	}
	return c.node, err
}

// NodeWithoutConstructing returns the underlying node variable
// so that clients may close it.
func (c *Context) NodeWithoutConstructing() *core.IpfsNode {
	return c.node
}

// Request represents a call to a command from a consumer
type Request interface {
	Path() []string
	Option(name string) *OptionValue
	Options() optMap
	SetOption(name string, val interface{})
	SetOptions(opts map[string]interface{}) error
	Arguments() []string
	SetArguments([]string)
	Files() File
	SetFiles(File)
	Context() *Context
	SetContext(Context)
	Command() *Command
	Cleanup() error

	ConvertOptions() error
}

type request struct {
	path       []string
	options    optMap
	arguments  []string
	files      File
	cmd        *Command
	ctx        Context
	optionDefs map[string]Option
}

// Path returns the command path of this request
func (r *request) Path() []string {
	return r.path
}

// Option returns the value of the option for given name.
func (r *request) Option(name string) *OptionValue {
	val, found := r.options[name]
	if found {
		return &OptionValue{val, found}
	}

	// if a value isn't defined for that name, we will try to look it up by its aliases

	// find the option with the specified name
	option, found := r.optionDefs[name]
	if !found {
		return nil
	}

	// try all the possible names, break if we find a value
	for _, n := range option.Names {
		val, found = r.options[n]
		if found {
			return &OptionValue{val, found}
		}
	}

	// MAYBE_TODO: use default value instead of nil
	return &OptionValue{nil, false}
}

// Options returns a copy of the option map
func (r *request) Options() optMap {
	output := make(optMap)
	for k, v := range r.options {
		output[k] = v
	}
	return output
}

// SetOption sets the value of the option for given name.
func (r *request) SetOption(name string, val interface{}) {
	// find the option with the specified name
	option, found := r.optionDefs[name]
	if !found {
		return
	}

	// try all the possible names, if we already have a value then set over it
	for _, n := range option.Names {
		_, found := r.options[n]
		if found {
			r.options[n] = val
			return
		}
	}

	r.options[name] = val
}

// SetOptions sets the option values, unsetting any values that were previously set
func (r *request) SetOptions(opts map[string]interface{}) error {
	r.options = opts
	return r.ConvertOptions()
}

// Arguments returns the arguments slice
func (r *request) Arguments() []string {
	return r.arguments
}

func (r *request) SetArguments(args []string) {
	r.arguments = args
}

func (r *request) Files() File {
	return r.files
}

func (r *request) SetFiles(f File) {
	r.files = f
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

func (r *request) Cleanup() error {
	// TODO
	return nil
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
		val, err := strconv.ParseInt(v, 0, 32)
		if err != nil {
			return nil, err
		}
		return int(val), err
	},
	Uint: func(v string) (interface{}, error) {
		val, err := strconv.ParseUint(v, 0, 32)
		if err != nil {
			return nil, err
		}
		return int(val), err
	},
	Float: func(v string) (interface{}, error) {
		return strconv.ParseFloat(v, 64)
	},
}

func (r *request) ConvertOptions() error {
	for k, v := range r.options {
		opt, ok := r.optionDefs[k]
		if !ok {
			continue
		}

		kind := reflect.TypeOf(v).Kind()
		if kind != opt.Type {
			if kind == String {
				convert := converters[opt.Type]
				str, ok := v.(string)
				if !ok {
					return u.ErrCast()
				}
				val, err := convert(str)
				if err != nil {
					value := fmt.Sprintf("value '%v'", v)
					if len(str) == 0 {
						value = "empty value"
					}
					return fmt.Errorf("Could not convert %s to type '%s' (for option '-%s')",
						value, opt.Type.String(), k)
				}
				r.options[k] = val

			} else {
				return fmt.Errorf("Option '%s' should be type '%s', but got type '%s'",
					k, opt.Type.String(), kind.String())
			}
		} else {
			r.options[k] = v
		}

		for _, name := range opt.Names {
			if _, ok := r.options[name]; name != k && ok {
				return fmt.Errorf("Duplicate command options were provided ('%s' and '%s')",
					k, name)
			}
		}
	}

	return nil
}

// NewEmptyRequest initializes an empty request
func NewEmptyRequest() (Request, error) {
	return NewRequest(nil, nil, nil, nil, nil, nil)
}

// NewRequest returns a request initialized with given arguments
// An non-nil error will be returned if the provided option values are invalid
func NewRequest(path []string, opts optMap, args []string, file File, cmd *Command, optDefs map[string]Option) (Request, error) {
	if path == nil {
		path = make([]string, 0)
	}
	if opts == nil {
		opts = make(map[string]interface{})
	}
	if args == nil {
		args = make([]string, 0)
	}
	if optDefs == nil {
		optDefs = make(map[string]Option)
	}

	req := &request{path, opts, args, file, cmd, Context{}, optDefs}
	err := req.ConvertOptions()
	if err != nil {
		return nil, err
	}

	return req, nil
}
