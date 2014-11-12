package commands

import (
	"errors"
	"reflect"
)

// Types of Command options
const (
	Invalid = reflect.Invalid
	Bool    = reflect.Bool
	Int     = reflect.Int
	Uint    = reflect.Uint
	Float   = reflect.Float64
	String  = reflect.String
)

// Option is used to specify a field that will be provided by a consumer
type Option struct {
	Names       []string     // a list of unique names to
	Type        reflect.Kind // value must be this type
	Description string       // a short string to describe this option

	// MAYBE_TODO: add more features(?):
	//Default interface{} // the default value (ignored if `Required` is true)
	//Required bool       // whether or not the option must be provided
}

// constructor helper functions
func NewOption(kind reflect.Kind, names ...string) Option {
	if len(names) < 2 {
		// FIXME(btc) don't panic (fix_before_merge)
		panic("Options require at least two string values (name and description)")
	}

	desc := names[len(names)-1]
	names = names[:len(names)-1]

	return Option{
		Names:       names,
		Type:        kind,
		Description: desc,
	}
}

// TODO handle description separately. this will take care of the panic case in
// NewOption

// For all func {Type}Option(...string) functions, the last variadic argument
// is treated as the description field.

func BoolOption(names ...string) Option {
	return NewOption(Bool, names...)
}
func IntOption(names ...string) Option {
	return NewOption(Int, names...)
}
func UintOption(names ...string) Option {
	return NewOption(Uint, names...)
}
func FloatOption(names ...string) Option {
	return NewOption(Float, names...)
}
func StringOption(names ...string) Option {
	return NewOption(String, names...)
}

type OptionValue struct {
	value interface{}
	found bool
}

// Found returns true if the option value was provided by the user (not a default value)
func (ov OptionValue) Found() bool {
	return ov.found
}

// value accessor methods, gets the value as a certain type
func (ov OptionValue) Bool() (bool, error) {
	val, ok := ov.value.(bool)
	if !ok {
		var err error
		if ov.value != nil {
			err = errors.New("error casting to bool")
		}
		return false, err
	}
	return val, nil
}
func (ov OptionValue) Int() (int, error) {
	val, ok := ov.value.(int)
	if !ok {
		var err error
		if ov.value != nil {
			err = errors.New("error casting to int")
		}
		return 0, err
	}
	return val, nil
}
func (ov OptionValue) Uint() (uint, error) {
	val, ok := ov.value.(uint)
	if !ok {
		var err error
		if ov.value != nil {
			err = errors.New("error casting to uint")
		}
		return 0, err
	}
	return val, nil
}
func (ov OptionValue) Float() (float64, error) {
	val, ok := ov.value.(float64)
	if !ok {
		var err error
		if ov.value != nil {
			err = errors.New("error casting to float64")
		}
		return 0.0, err
	}
	return val, nil
}
func (ov OptionValue) String() (string, error) {
	val, ok := ov.value.(string)
	if !ok {
		var err error
		if ov.value != nil {
			err = errors.New("error casting to string")
		}
		return "", err
	}
	return val, nil
}

// Flag names
const (
	EncShort = "enc"
	EncLong  = "encoding"
)

// options that are used by this package
var globalOptions = []Option{
	Option{[]string{EncShort, EncLong}, String,
		"The encoding type the output should be encoded with (json, xml, or text)"},
}

// the above array of Options, wrapped in a Command
var globalCommand = &Command{
	Options: globalOptions,
}
