package commands

import (
	"fmt"
	"reflect"
	"strconv"
)

// Request represents a call to a command from a consumer
type Request struct {
	path			[]string
	options   map[string]interface{}
	arguments []string
}

func (r *Request) Path() []string {
	return r.path
}

func (r *Request) SetPath(path []string) {
	r.path = path
}

func (r *Request) Option(name string) interface{} {
	return r.options[name]
}

func (r *Request) SetOption(name string, value interface{}) {
	r.options[name] = value
}

func (r *Request) Arguments() []string {
	return r.arguments
}

type converter func(string)(interface{}, error)
var converters map[reflect.Kind]converter = map[reflect.Kind]converter{
	Bool: func(v string)(interface{}, error) {
		if v == "" {
			return true, nil
		}
		return strconv.ParseBool(v)
	},
	Int: func(v string)(interface{}, error) {
		return strconv.ParseInt(v, 0, 32)
	},
	Uint: func(v string)(interface{}, error) {
		return strconv.ParseInt(v, 0, 32)
	},
	Float: func(v string)(interface{}, error) {
		return strconv.ParseFloat(v, 64)
	},
}

func (r *Request) convertOptions(options map[string]Option) error {
	converted := make(map[string]interface{})

	for k, v := range r.options {
		opt, ok := options[k]
		if !ok {
			return fmt.Errorf("Unrecognized option: '%s'", k)
		}

		kind := reflect.TypeOf(v).Kind()
		var value interface{}

		if kind != opt.Type {
			if kind == String {
				convert := converters[opt.Type]
				val, err := convert(v.(string))
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

func NewRequest() *Request {
	return &Request{
		make([]string, 0),
		make(map[string]interface{}),
		make([]string, 0),
	}
}
