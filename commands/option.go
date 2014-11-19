package commands

import (
	"reflect"

	"github.com/jbenet/go-ipfs/util"
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
type Option interface {
	Names() []string     // a list of unique names matched with user-provided flags
	Type() reflect.Kind  // value must be this type
	Description() string // a short string that describes this option
}

type option struct {
	names       []string
	kind        reflect.Kind
	description string
}

func (o *option) Names() []string {
	return o.names
}

func (o *option) Type() reflect.Kind {
	return o.kind
}

func (o *option) Description() string {
	return o.description
}

// constructor helper functions
func NewOption(kind reflect.Kind, names ...string) Option {
	if len(names) < 2 {
		// FIXME(btc) don't panic (fix_before_merge)
		panic("Options require at least two string values (name and description)")
	}

	desc := names[len(names)-1]
	names = names[:len(names)-1]

	return &option{
		names:       names,
		kind:        kind,
		description: desc,
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
func (ov OptionValue) Bool() (value bool, found bool, err error) {
	if !ov.found {
		return false, false, nil
	}
	val, ok := ov.value.(bool)
	if !ok {
		err = util.ErrCast()
	}
	return val, ov.found, err
}

func (ov OptionValue) Int() (value int, found bool, err error) {
	if !ov.found {
		return 0, false, nil
	}
	val, ok := ov.value.(int)
	if !ok {
		err = util.ErrCast()
	}
	return val, ov.found, err
}

func (ov OptionValue) Uint() (value uint, found bool, err error) {
	if !ov.found {
		return 0, false, nil
	}
	val, ok := ov.value.(uint)
	if !ok {
		err = util.ErrCast()
	}
	return val, ov.found, err
}

func (ov OptionValue) Float() (value float64, found bool, err error) {
	if !ov.found {
		return 0, false, nil
	}
	val, ok := ov.value.(float64)
	if !ok {
		err = util.ErrCast()
	}
	return val, ov.found, err
}

func (ov OptionValue) String() (value string, found bool, err error) {
	if !ov.found {
		return "", false, nil
	}
	val, ok := ov.value.(string)
	if !ok {
		err = util.ErrCast()
	}
	return val, ov.found, err
}

// Flag names
const (
	EncShort = "enc"
	EncLong  = "encoding"
	RecShort = "r"
	RecLong  = "recursive"
)

// options that are used by this package
var OptionEncodingType = StringOption(EncShort, EncLong, "The encoding type the output should be encoded with (json, xml, or text)")
var OptionRecursivePath = BoolOption(RecShort, RecLong, "Add directory paths recursively")

// global options, added to every command
var globalOptions = []Option{
	OptionEncodingType,
}

// the above array of Options, wrapped in a Command
var globalCommand = &Command{
	Options: globalOptions,
}
