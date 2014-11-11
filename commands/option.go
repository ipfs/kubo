package commands

import "reflect"

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
		panic("Options require at least two string values (name and description)")
	}

	desc := names[len(names)-1]
	names = names[:len(names)-2]

	return Option{
		Names:       names,
		Type:        kind,
		Description: desc,
	}
}

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
