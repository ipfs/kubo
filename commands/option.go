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

	// TODO: add more features(?):
	//Default interface{} // the default value (ignored if `Required` is true)
	//Required bool       // whether or not the option must be provided
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
