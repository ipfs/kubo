package config

import (
	"encoding"
	"encoding/json"
	"fmt"
	"time"
)

// Strings is a helper type that (un)marshals a single string to/from a single
// JSON string and a slice of strings to/from a JSON array of strings.
type Strings []string

// UnmarshalJSON conforms to the json.Unmarshaler interface.
func (o *Strings) UnmarshalJSON(data []byte) error {
	if data[0] == '[' {
		return json.Unmarshal(data, (*[]string)(o))
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if len(value) == 0 {
		*o = []string{}
	} else {
		*o = []string{value}
	}
	return nil
}

// MarshalJSON conforms to the json.Marshaler interface.
func (o Strings) MarshalJSON() ([]byte, error) {
	switch len(o) {
	case 0:
		return json.Marshal(nil)
	case 1:
		return json.Marshal(o[0])
	default:
		return json.Marshal([]string(o))
	}
}

var _ json.Unmarshaler = (*Strings)(nil)
var _ json.Marshaler = (*Strings)(nil)

// Flag represents a ternary value: false (-1), default (0), or true (+1).
//
// When encoded in json, False is "false", Default is "null" (or empty), and True
// is "true".
type Flag int8

const (
	False   Flag = -1
	Default Flag = 0
	True    Flag = 1
)

// WithDefault resolves the value of the flag given the provided default value.
//
// Panics if Flag is an invalid value.
func (f Flag) WithDefault(defaultValue bool) bool {
	switch f {
	case False:
		return false
	case Default:
		return defaultValue
	case True:
		return true
	default:
		panic(fmt.Sprintf("invalid flag value %d", f))
	}
}

func (f Flag) MarshalJSON() ([]byte, error) {
	switch f {
	case Default:
		return json.Marshal(nil)
	case True:
		return json.Marshal(true)
	case False:
		return json.Marshal(false)
	default:
		return nil, fmt.Errorf("invalid flag value: %d", f)
	}
}

func (f *Flag) UnmarshalJSON(input []byte) error {
	switch string(input) {
	case "null":
		*f = Default
	case "false":
		*f = False
	case "true":
		*f = True
	default:
		return fmt.Errorf("failed to unmarshal %q into a flag: must be null/undefined, true, or false", string(input))
	}
	return nil
}

func (f Flag) String() string {
	switch f {
	case Default:
		return "default"
	case True:
		return "true"
	case False:
		return "false"
	default:
		return fmt.Sprintf("<invalid flag value %d>", f)
	}
}

var _ json.Unmarshaler = (*Flag)(nil)
var _ json.Marshaler = (*Flag)(nil)

// Priority represents a value with a priority where 0 means "default" and -1
// means "disabled".
//
// When encoded in json, Default is encoded as "null" and Disabled is encoded as
// "false".
type Priority int64

const (
	DefaultPriority Priority = 0
	Disabled        Priority = -1
)

// WithDefault resolves the priority with the given default.
//
// If defaultPriority is Default/0, this function will return 0.
//
// Panics if the priority has an invalid value (e.g., not DefaultPriority,
// Disabled, or > 0).
func (p Priority) WithDefault(defaultPriority Priority) (priority int64, enabled bool) {
	switch p {
	case Disabled:
		return 0, false
	case DefaultPriority:
		switch defaultPriority {
		case Disabled:
			return 0, false
		case DefaultPriority:
			return 0, true
		default:
			if defaultPriority <= 0 {
				panic(fmt.Sprintf("invalid priority %d < 0", int64(defaultPriority)))
			}
			return int64(defaultPriority), true
		}
	default:
		if p <= 0 {
			panic(fmt.Sprintf("invalid priority %d < 0", int64(p)))
		}
		return int64(p), true
	}
}

func (p Priority) MarshalJSON() ([]byte, error) {
	// > 0 == Priority
	if p > 0 {
		return json.Marshal(int64(p))
	}
	// <= 0 == special
	switch p {
	case DefaultPriority:
		return json.Marshal(nil)
	case Disabled:
		return json.Marshal(false)
	default:
		return nil, fmt.Errorf("invalid priority value: %d", p)
	}
}

func (p *Priority) UnmarshalJSON(input []byte) error {
	switch string(input) {
	case "null", "undefined":
		*p = DefaultPriority
	case "false":
		*p = Disabled
	case "true":
		return fmt.Errorf("'true' is not a valid priority")
	default:
		var priority int64
		err := json.Unmarshal(input, &priority)
		if err != nil {
			return err
		}
		if priority <= 0 {
			return fmt.Errorf("priority must be positive: %d <= 0", priority)
		}
		*p = Priority(priority)
	}
	return nil
}

func (p Priority) String() string {
	if p > 0 {
		return fmt.Sprintf("%d", p)
	}
	switch p {
	case DefaultPriority:
		return "default"
	case Disabled:
		return "false"
	default:
		return fmt.Sprintf("<invalid priority %d>", p)
	}
}

var _ json.Unmarshaler = (*Flag)(nil)
var _ json.Marshaler = (*Flag)(nil)

// Duration wraps time.Duration to provide json serialization and deserialization.
//
// NOTE: the zero value encodes to an empty string.
type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	*d = Duration(dur)
	return err
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

var _ encoding.TextUnmarshaler = (*Duration)(nil)
var _ encoding.TextMarshaler = (*Duration)(nil)
