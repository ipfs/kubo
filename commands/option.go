package commands

import (
	"reflect"

	"github.com/ipfs/go-ipfs/util"
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
	LongName() string    // long option name
	ShortName() rune     // short option name
	Type() reflect.Kind  // value must be this type
	Description() string // a short string that describes this option
}

type option struct {
	longName    string
	shortName   rune
	kind        reflect.Kind
	description string
}

func (o *option) LongName() string {
	return o.longName
}

func (o *option) ShortName() rune {
	return o.shortName
}

func (o *option) Type() reflect.Kind {
	return o.kind
}

func (o *option) Description() string {
	return o.description
}

// constructor helper functions
func NewOption(kind reflect.Kind, longName string, shortName rune, desc string) Option {

	return &option{
		longName:    longName,
		shortName:   shortName,
		kind:        kind,
		description: desc,
	}
}

// TODO handle description separately. this will take care of the panic case in
// NewOption

// For all func {Type}Option(...string) functions, the last variadic argument
// is treated as the description field.

func BoolOption(longName string, shortName rune, desc string) Option {
	return NewOption(Bool, longName, shortName, desc)
}
func IntOption(longName string, shortName rune, desc string) Option {
	return NewOption(Int, longName, shortName, desc)
}
func UintOption(longName string, shortName rune, desc string) Option {
	return NewOption(Uint, longName, shortName, desc)
}
func FloatOption(longName string, shortName rune, desc string) Option {
	return NewOption(Float, longName, shortName, desc)
}
func StringOption(longName string, shortName rune, desc string) Option {
	return NewOption(String, longName, shortName, desc)
}

type OptionValue struct {
	value interface{}
	found bool
	def   Option
}

// Found returns true if the option value was provided by the user (not a default value)
func (ov OptionValue) Found() bool {
	return ov.found
}

// Definition returns the option definition for the provided value
func (ov OptionValue) Definition() Option {
	return ov.def
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
	EncShort   = 'E'
	EncLong    = "encoding"
	RecShort   = 'r'
	RecLong    = "recursive"
	ChanOpt    = "stream-channels"
	TimeoutOpt = "timeout"
)

// options that are used by this package
var OptionEncodingType = StringOption(EncLong, EncShort, "The encoding type the output should be encoded with (json, xml, or text)")
var OptionRecursivePath = BoolOption(RecLong, RecShort, "Add directory paths recursively")
var OptionStreamChannels = BoolOption(ChanOpt, 0, "Stream channel output")
var OptionTimeout = StringOption(TimeoutOpt, 0, "set a global timeout on the command")

// global options, added to every command
var globalOptions = []Option{
	OptionEncodingType,
	OptionStreamChannels,
	OptionTimeout,
}

// the above array of Options, wrapped in a Command
var globalCommand = &Command{
	Options: globalOptions,
}
