package config

import (
	"encoding/json"
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
