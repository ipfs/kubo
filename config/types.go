package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

var (
	_ json.Unmarshaler = (*Strings)(nil)
	_ json.Marshaler   = (*Strings)(nil)
)

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

var (
	_ json.Unmarshaler = (*Flag)(nil)
	_ json.Marshaler   = (*Flag)(nil)
)

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

var (
	_ json.Unmarshaler = (*Priority)(nil)
	_ json.Marshaler   = (*Priority)(nil)
)

// OptionalDuration wraps time.Duration to provide json serialization and deserialization.
//
// NOTE: the zero value encodes to JSON nill.
type OptionalDuration struct {
	value *time.Duration
}

// NewOptionalDuration returns an OptionalDuration from a string.
func NewOptionalDuration(d time.Duration) *OptionalDuration {
	return &OptionalDuration{value: &d}
}

func (d *OptionalDuration) UnmarshalJSON(input []byte) error {
	switch string(input) {
	case "null", "undefined", "\"null\"", "", "default", "\"\"", "\"default\"":
		*d = OptionalDuration{}
		return nil
	default:
		text := strings.Trim(string(input), "\"")
		value, err := time.ParseDuration(text)
		if err != nil {
			return err
		}
		*d = OptionalDuration{value: &value}
		return nil
	}
}

func (d *OptionalDuration) IsDefault() bool {
	return d == nil || d.value == nil
}

func (d *OptionalDuration) WithDefault(defaultValue time.Duration) time.Duration {
	if d == nil || d.value == nil {
		return defaultValue
	}
	return *d.value
}

func (d OptionalDuration) MarshalJSON() ([]byte, error) {
	if d.value == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(d.value.String())
}

func (d OptionalDuration) String() string {
	if d.value == nil {
		return "default"
	}
	return d.value.String()
}

var (
	_ json.Unmarshaler = (*OptionalDuration)(nil)
	_ json.Marshaler   = (*OptionalDuration)(nil)
)

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unable to parse duration, expected a duration string or a float, but got %T", v)
	}
}

var (
	_ json.Unmarshaler = (*Duration)(nil)
	_ json.Marshaler   = (*Duration)(nil)
)

// OptionalInteger represents an integer that has a default value
//
// When encoded in json, Default is encoded as "null".
type OptionalInteger struct {
	value *int64
}

// NewOptionalInteger returns an OptionalInteger from a int64.
func NewOptionalInteger(v int64) *OptionalInteger {
	return &OptionalInteger{value: &v}
}

// WithDefault resolves the integer with the given default.
func (p *OptionalInteger) WithDefault(defaultValue int64) (value int64) {
	if p == nil || p.value == nil {
		return defaultValue
	}
	return *p.value
}

// IsDefault returns if this is a default optional integer.
func (p *OptionalInteger) IsDefault() bool {
	return p == nil || p.value == nil
}

func (p OptionalInteger) MarshalJSON() ([]byte, error) {
	if p.value != nil {
		return json.Marshal(p.value)
	}
	return json.Marshal(nil)
}

func (p *OptionalInteger) UnmarshalJSON(input []byte) error {
	switch string(input) {
	case "null", "undefined":
		*p = OptionalInteger{}
	default:
		var value int64
		err := json.Unmarshal(input, &value)
		if err != nil {
			return err
		}
		*p = OptionalInteger{value: &value}
	}
	return nil
}

func (p OptionalInteger) String() string {
	if p.value == nil {
		return "default"
	}
	return fmt.Sprintf("%d", *p.value)
}

var (
	_ json.Unmarshaler = (*OptionalInteger)(nil)
	_ json.Marshaler   = (*OptionalInteger)(nil)
)

// OptionalString represents a string that has a default value
//
// When encoded in json, Default is encoded as "null".
type OptionalString struct {
	value *string
}

// NewOptionalString returns an OptionalString from a string.
func NewOptionalString(s string) *OptionalString {
	return &OptionalString{value: &s}
}

// WithDefault resolves the integer with the given default.
func (p *OptionalString) WithDefault(defaultValue string) (value string) {
	if p == nil || p.value == nil {
		return defaultValue
	}
	return *p.value
}

// IsDefault returns if this is a default optional integer.
func (p *OptionalString) IsDefault() bool {
	return p == nil || p.value == nil
}

func (p OptionalString) MarshalJSON() ([]byte, error) {
	if p.value != nil {
		return json.Marshal(p.value)
	}
	return json.Marshal(nil)
}

func (p *OptionalString) UnmarshalJSON(input []byte) error {
	switch string(input) {
	case "null", "undefined":
		*p = OptionalString{}
	default:
		var value string
		err := json.Unmarshal(input, &value)
		if err != nil {
			return err
		}
		*p = OptionalString{value: &value}
	}
	return nil
}

func (p OptionalString) String() string {
	if p.value == nil {
		return "default"
	}
	return *p.value
}

var (
	_ json.Unmarshaler = (*OptionalInteger)(nil)
	_ json.Marshaler   = (*OptionalInteger)(nil)
)

type swarmLimits doNotUse

var _ json.Unmarshaler = swarmLimits(false)

func (swarmLimits) UnmarshalJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	for {
		switch tok, err := d.Token(); err {
		case io.EOF:
			return nil
		case nil:
			switch tok {
			case json.Delim('{'), json.Delim('}'):
				// accept empty objects
				continue
			}
			//nolint
			return fmt.Errorf("The Swarm.ResourceMgr.Limits configuration has been removed in Kubo 0.19 and should be empty or not present. To set custom libp2p limits, read https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#user-supplied-override-limits")
		default:
			return err
		}
	}
}

type experimentalAcceleratedDHTClient doNotUse

var _ json.Unmarshaler = experimentalAcceleratedDHTClient(false)

func (experimentalAcceleratedDHTClient) UnmarshalJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	for {
		switch tok, err := d.Token(); err {
		case io.EOF:
			return nil
		case nil:
			switch tok {
			case json.Delim('{'), json.Delim('}'):
				// accept empty objects
				continue
			}
			//nolint
			return fmt.Errorf("The Experimental.AcceleratedDHTClient key has been moved to Routing.AcceleratedDHTClient in Kubo 0.21, please use this new key and remove the old one.")
		default:
			return err
		}
	}
}

// doNotUse is a type you must not use, it should be struct{} but encoding/json
// does not support omitempty on structs and I can't be bothered to write custom
// marshalers on all structs that have a doNotUse field.
type doNotUse bool

type graphsyncEnabled doNotUse

var _ json.Unmarshaler = graphsyncEnabled(false)

func (graphsyncEnabled) UnmarshalJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	for {
		switch tok, err := d.Token(); err {
		case io.EOF:
			return nil
		case nil:
			switch tok {
			case json.Delim('{'), json.Delim('}'), false:
				// accept empty objects and false
				continue
			}
			//nolint
			return fmt.Errorf("Support for Experimental.GraphsyncEnabled has been removed in Kubo 0.25.0, please remove this key. For more details see https://github.com/ipfs/kubo/pull/9747.")
		default:
			return err
		}
	}
}
