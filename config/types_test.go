package config

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionalDuration(t *testing.T) {
	makeDurationPointer := func(d time.Duration) *time.Duration { return &d }

	t.Run("marshalling and unmarshalling", func(t *testing.T) {
		out, err := json.Marshal(OptionalDuration{value: makeDurationPointer(time.Second)})
		if err != nil {
			t.Fatal(err)
		}
		expected := "\"1s\""
		if string(out) != expected {
			t.Fatalf("expected %s, got %s", expected, string(out))
		}
		var d OptionalDuration

		if err := json.Unmarshal(out, &d); err != nil {
			t.Fatal(err)
		}
		if *d.value != time.Second {
			t.Fatal("expected a second")
		}
	})

	t.Run("default value", func(t *testing.T) {
		for _, jsonStr := range []string{"null", "\"null\"", "\"\"", "\"default\""} {
			var d OptionalDuration
			if !d.IsDefault() {
				t.Fatal("expected value to be the default initially")
			}
			if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
				t.Fatalf("%s failed to unmarshall with %s", jsonStr, err)
			}
			if dur := d.WithDefault(time.Hour); dur != time.Hour {
				t.Fatalf("expected default value to be used, got %s", dur)
			}
			if !d.IsDefault() {
				t.Fatal("expected value to be the default")
			}
		}
	})

	t.Run("omitempty with default value", func(t *testing.T) {
		type Foo struct {
			D *OptionalDuration `json:",omitempty"`
		}
		// marshall to JSON without empty field
		out, err := json.Marshal(new(Foo))
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "{}" {
			t.Fatalf("expected omitempty to omit the duration, got %s", out)
		}
		// unmarshall missing value and get the default
		var foo2 Foo
		if err := json.Unmarshal(out, &foo2); err != nil {
			t.Fatalf("%s failed to unmarshall with %s", string(out), err)
		}
		if dur := foo2.D.WithDefault(time.Hour); dur != time.Hour {
			t.Fatalf("expected default value to be used, got %s", dur)
		}
		if !foo2.D.IsDefault() {
			t.Fatal("expected value to be the default")
		}
	})

	t.Run("roundtrip including the default values", func(t *testing.T) {
		for jsonStr, goValue := range map[string]OptionalDuration{
			// there are various footguns user can hit, normalize them to the canonical default
			"null":        {}, // JSON null → default value
			"\"null\"":    {}, // JSON string "null" sent/set by "ipfs config" cli → default value
			"\"default\"": {}, // explicit "default" as string
			"\"\"":        {}, // user removed custom value, empty string should also parse as default
			"\"1s\"":      {value: makeDurationPointer(time.Second)},
			"\"42h1m3s\"": {value: makeDurationPointer(42*time.Hour + 1*time.Minute + 3*time.Second)},
		} {
			var d OptionalDuration
			err := json.Unmarshal([]byte(jsonStr), &d)
			if err != nil {
				t.Fatal(err)
			}

			if goValue.value == nil && d.value == nil {
			} else if goValue.value == nil && d.value != nil {
				t.Errorf("expected nil for %s, got %s", jsonStr, d)
			} else if *d.value != *goValue.value {
				t.Fatalf("expected %s for %s, got %s", goValue, jsonStr, d)
			}

			// Test Reverse
			out, err := json.Marshal(goValue)
			if err != nil {
				t.Fatal(err)
			}
			if goValue.value == nil {
				if !bytes.Equal(out, []byte("null")) {
					t.Fatalf("expected JSON null for %s, got %s", jsonStr, string(out))
				}
				continue
			}
			if string(out) != jsonStr {
				t.Fatalf("expected %s, got %s", jsonStr, string(out))
			}
		}
	})

	t.Run("invalid duration values", func(t *testing.T) {
		for _, invalid := range []string{
			"\"s\"", "\"1ę\"", "\"-1\"", "\"1H\"", "\"day\"",
		} {
			var d OptionalDuration
			err := json.Unmarshal([]byte(invalid), &d)
			if err == nil {
				t.Errorf("expected to fail to decode %s as an OptionalDuration, got %s instead", invalid, d)
			}
		}
	})
}

func TestOneStrings(t *testing.T) {
	out, err := json.Marshal(Strings{"one"})
	if err != nil {
		t.Fatal(err)
	}
	expected := "\"one\""
	if string(out) != expected {
		t.Fatalf("expected %s, got %s", expected, string(out))
	}
}

func TestNoStrings(t *testing.T) {
	out, err := json.Marshal(Strings{})
	if err != nil {
		t.Fatal(err)
	}
	expected := "null"
	if string(out) != expected {
		t.Fatalf("expected %s, got %s", expected, string(out))
	}
}

func TestManyStrings(t *testing.T) {
	out, err := json.Marshal(Strings{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	expected := "[\"one\",\"two\"]"
	if string(out) != expected {
		t.Fatalf("expected %s, got %s", expected, string(out))
	}
}

func TestFunkyStrings(t *testing.T) {
	toParse := " [   \"one\",   \"two\" ]  "
	var s Strings
	if err := json.Unmarshal([]byte(toParse), &s); err != nil {
		t.Fatal(err)
	}
	if len(s) != 2 || s[0] != "one" && s[1] != "two" {
		t.Fatalf("unexpected result: %v", s)
	}
}

func TestFlag(t *testing.T) {
	// make sure we have the right zero value.
	var defaultFlag Flag
	if defaultFlag != Default {
		t.Errorf("expected default flag to be %q, got %q", Default, defaultFlag)
	}

	if defaultFlag.WithDefault(true) != true {
		t.Error("expected default & true to be true")
	}

	if defaultFlag.WithDefault(false) != false {
		t.Error("expected default & false to be false")
	}

	if True.WithDefault(false) != true {
		t.Error("default should only apply to default")
	}

	if False.WithDefault(true) != false {
		t.Error("default should only apply to default")
	}

	if True.WithDefault(true) != true {
		t.Error("true & true is true")
	}

	if False.WithDefault(true) != false {
		t.Error("false & false is false")
	}

	for jsonStr, goValue := range map[string]Flag{
		"null":  Default,
		"true":  True,
		"false": False,
	} {
		var d Flag
		err := json.Unmarshal([]byte(jsonStr), &d)
		if err != nil {
			t.Fatal(err)
		}
		if d != goValue {
			t.Fatalf("expected %s, got %s", goValue, d)
		}

		// Reverse
		out, err := json.Marshal(goValue)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != jsonStr {
			t.Fatalf("expected %s, got %s", jsonStr, string(out))
		}
	}

	type Foo struct {
		F Flag `json:",omitempty"`
	}
	out, err := json.Marshal(new(Foo))
	if err != nil {
		t.Fatal(err)
	}
	expected := "{}"
	if string(out) != expected {
		t.Fatal("expected omitempty to omit the flag")
	}
}

func TestPriority(t *testing.T) {
	// make sure we have the right zero value.
	var defaultPriority Priority
	if defaultPriority != DefaultPriority {
		t.Errorf("expected default priority to be %q, got %q", DefaultPriority, defaultPriority)
	}

	if _, ok := defaultPriority.WithDefault(Disabled); ok {
		t.Error("should have been disabled")
	}

	if p, ok := defaultPriority.WithDefault(1); !ok || p != 1 {
		t.Errorf("priority should have been 1, got %d", p)
	}

	if p, ok := defaultPriority.WithDefault(DefaultPriority); !ok || p != 0 {
		t.Errorf("priority should have been 0, got %d", p)
	}

	for jsonStr, goValue := range map[string]Priority{
		"null":  DefaultPriority,
		"false": Disabled,
		"1":     1,
		"2":     2,
		"100":   100,
	} {
		var d Priority
		err := json.Unmarshal([]byte(jsonStr), &d)
		if err != nil {
			t.Fatal(err)
		}
		if d != goValue {
			t.Fatalf("expected %s, got %s", goValue, d)
		}

		// Reverse
		out, err := json.Marshal(goValue)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != jsonStr {
			t.Fatalf("expected %s, got %s", jsonStr, string(out))
		}
	}

	type Foo struct {
		P Priority `json:",omitempty"`
	}
	out, err := json.Marshal(new(Foo))
	if err != nil {
		t.Fatal(err)
	}
	expected := "{}"
	if string(out) != expected {
		t.Fatal("expected omitempty to omit the flag")
	}
	for _, invalid := range []string{
		"0", "-1", "-2", "1.1", "0.0",
	} {
		var p Priority
		err := json.Unmarshal([]byte(invalid), &p)
		if err == nil {
			t.Errorf("expected to fail to decode %s as a priority", invalid)
		}
	}
}

func TestOptionalInteger(t *testing.T) {
	makeInt64Pointer := func(v int64) *int64 {
		return &v
	}

	var defaultOptionalInt OptionalInteger
	if !defaultOptionalInt.IsDefault() {
		t.Fatal("should be the default")
	}
	if val := defaultOptionalInt.WithDefault(0); val != 0 {
		t.Errorf("optional integer should have been 0, got %d", val)
	}

	if val := defaultOptionalInt.WithDefault(1); val != 1 {
		t.Errorf("optional integer should have been 1, got %d", val)
	}

	if val := defaultOptionalInt.WithDefault(-1); val != -1 {
		t.Errorf("optional integer should have been -1, got %d", val)
	}

	var filledInt OptionalInteger
	filledInt = OptionalInteger{value: makeInt64Pointer(1)}
	if filledInt.IsDefault() {
		t.Fatal("should not be the default")
	}
	if val := filledInt.WithDefault(0); val != 1 {
		t.Errorf("optional integer should have been 1, got %d", val)
	}

	if val := filledInt.WithDefault(-1); val != 1 {
		t.Errorf("optional integer should have been 1, got %d", val)
	}

	filledInt = OptionalInteger{value: makeInt64Pointer(0)}
	if val := filledInt.WithDefault(1); val != 0 {
		t.Errorf("optional integer should have been 0, got %d", val)
	}

	for jsonStr, goValue := range map[string]OptionalInteger{
		"null": {},
		"0":    {value: makeInt64Pointer(0)},
		"1":    {value: makeInt64Pointer(1)},
		"-1":   {value: makeInt64Pointer(-1)},
	} {
		var d OptionalInteger
		err := json.Unmarshal([]byte(jsonStr), &d)
		if err != nil {
			t.Fatal(err)
		}

		if goValue.value == nil && d.value == nil {
		} else if goValue.value == nil && d.value != nil {
			t.Errorf("expected default, got %s", d)
		} else if *d.value != *goValue.value {
			t.Fatalf("expected %s, got %s", goValue, d)
		}

		// Reverse
		out, err := json.Marshal(goValue)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != jsonStr {
			t.Fatalf("expected %s, got %s", jsonStr, string(out))
		}
	}

	// marshal with omitempty
	type Foo struct {
		I *OptionalInteger `json:",omitempty"`
	}
	out, err := json.Marshal(new(Foo))
	if err != nil {
		t.Fatal(err)
	}
	expected := "{}"
	if string(out) != expected {
		t.Fatal("expected omitempty to omit the optional integer")
	}

	// unmarshal from omitempty output and get default value
	var foo2 Foo
	if err := json.Unmarshal(out, &foo2); err != nil {
		t.Fatalf("%s failed to unmarshall with %s", string(out), err)
	}
	if i := foo2.I.WithDefault(42); i != 42 {
		t.Fatalf("expected default value to be used, got %d", i)
	}
	if !foo2.I.IsDefault() {
		t.Fatal("expected value to be the default")
	}

	// test invalid values
	for _, invalid := range []string{
		"foo", "-1.1", "1.1", "0.0", "[]",
	} {
		var p OptionalInteger
		err := json.Unmarshal([]byte(invalid), &p)
		if err == nil {
			t.Errorf("expected to fail to decode %s as a priority", invalid)
		}
	}
}

func TestOptionalString(t *testing.T) {
	makeStringPointer := func(v string) *string {
		return &v
	}

	var defaultOptionalString OptionalString
	if !defaultOptionalString.IsDefault() {
		t.Fatal("should be the default")
	}
	if val := defaultOptionalString.WithDefault(""); val != "" {
		t.Errorf("optional string should have been empty, got %s", val)
	}
	if val := defaultOptionalString.String(); val != "default" {
		t.Fatalf("default optional string should be the 'default' string, got %s", val)
	}
	if val := defaultOptionalString.WithDefault("foo"); val != "foo" {
		t.Errorf("optional string should have been foo, got %s", val)
	}

	var filledStr OptionalString
	filledStr = OptionalString{value: makeStringPointer("foo")}
	if filledStr.IsDefault() {
		t.Fatal("should not be the default")
	}
	if val := filledStr.WithDefault("bar"); val != "foo" {
		t.Errorf("optional string should have been foo, got %s", val)
	}
	if val := filledStr.String(); val != "foo" {
		t.Fatalf("optional string should have been foo, got %s", val)
	}
	filledStr = OptionalString{value: makeStringPointer("")}
	if val := filledStr.WithDefault("foo"); val != "" {
		t.Errorf("optional string should have been 0, got %s", val)
	}

	for jsonStr, goValue := range map[string]OptionalString{
		"null":     {},
		"\"0\"":    {value: makeStringPointer("0")},
		"\"\"":     {value: makeStringPointer("")},
		`"1"`:      {value: makeStringPointer("1")},
		`"-1"`:     {value: makeStringPointer("-1")},
		`"qwerty"`: {value: makeStringPointer("qwerty")},
	} {
		var d OptionalString
		err := json.Unmarshal([]byte(jsonStr), &d)
		if err != nil {
			t.Fatal(err)
		}

		if goValue.value == nil && d.value == nil {
		} else if goValue.value == nil && d.value != nil {
			t.Errorf("expected default, got %s", d)
		} else if *d.value != *goValue.value {
			t.Fatalf("expected %s, got %s", goValue, d)
		}

		// Reverse
		out, err := json.Marshal(goValue)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != jsonStr {
			t.Fatalf("expected %s, got %s", jsonStr, string(out))
		}
	}

	// marshal with omitempty
	type Foo struct {
		S *OptionalString `json:",omitempty"`
	}
	out, err := json.Marshal(new(Foo))
	if err != nil {
		t.Fatal(err)
	}
	expected := "{}"
	if string(out) != expected {
		t.Fatal("expected omitempty to omit the optional integer")
	}
	// unmarshal from omitempty output and get default value
	var foo2 Foo
	if err := json.Unmarshal(out, &foo2); err != nil {
		t.Fatalf("%s failed to unmarshall with %s", string(out), err)
	}
	if s := foo2.S.WithDefault("foo"); s != "foo" {
		t.Fatalf("expected default value to be used, got %s", s)
	}
	if !foo2.S.IsDefault() {
		t.Fatal("expected value to be the default")
	}

	for _, invalid := range []string{
		"[]", "{}", "0", "a", "'b'",
	} {
		var p OptionalString
		err := json.Unmarshal([]byte(invalid), &p)
		if err == nil {
			t.Errorf("expected to fail to decode %s as an optional string", invalid)
		}
	}
}

func TestOptionalBytes(t *testing.T) {
	makeStringPointer := func(v string) *string { return &v }

	t.Run("default value", func(t *testing.T) {
		var b OptionalBytes
		assert.True(t, b.IsDefault())
		assert.Equal(t, uint64(0), b.WithDefault(0))
		assert.Equal(t, uint64(1024), b.WithDefault(1024))
		assert.Equal(t, "default", b.String())
	})

	t.Run("non-default value", func(t *testing.T) {
		b := OptionalBytes{OptionalString{value: makeStringPointer("1MiB")}}
		assert.False(t, b.IsDefault())
		assert.Equal(t, uint64(1048576), b.WithDefault(512))
		assert.Equal(t, "1MiB", b.String())
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		testCases := []struct {
			jsonInput     string
			jsonOutput    string
			expectedValue string
		}{
			{"null", "null", ""},
			{"\"256KiB\"", "\"256KiB\"", "256KiB"},
			{"\"1MiB\"", "\"1MiB\"", "1MiB"},
			{"\"5GiB\"", "\"5GiB\"", "5GiB"},
			{"\"256KB\"", "\"256KB\"", "256KB"},
			{"1048576", "\"1048576\"", "1048576"},
		}

		for _, tc := range testCases {
			t.Run(tc.jsonInput, func(t *testing.T) {
				var b OptionalBytes
				err := json.Unmarshal([]byte(tc.jsonInput), &b)
				require.NoError(t, err)

				if tc.expectedValue == "" {
					assert.Nil(t, b.value)
				} else {
					require.NotNil(t, b.value)
					assert.Equal(t, tc.expectedValue, *b.value)
				}

				out, err := json.Marshal(b)
				require.NoError(t, err)
				assert.Equal(t, tc.jsonOutput, string(out))
			})
		}
	})

	t.Run("parsing byte sizes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected uint64
		}{
			{"256KiB", 262144},
			{"1MiB", 1048576},
			{"5GiB", 5368709120},
			{"256KB", 256000},
			{"1048576", 1048576},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				var b OptionalBytes
				err := json.Unmarshal([]byte("\""+tc.input+"\""), &b)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, b.WithDefault(0))
			})
		}
	})

	t.Run("omitempty", func(t *testing.T) {
		type Foo struct {
			B *OptionalBytes `json:",omitempty"`
		}

		out, err := json.Marshal(new(Foo))
		require.NoError(t, err)
		assert.Equal(t, "{}", string(out))

		var foo2 Foo
		err = json.Unmarshal(out, &foo2)
		require.NoError(t, err)

		if foo2.B != nil {
			assert.Equal(t, uint64(1024), foo2.B.WithDefault(1024))
			assert.True(t, foo2.B.IsDefault())
		} else {
			// When field is omitted, pointer is nil which is also considered default
			t.Log("B is nil, which is acceptable for omitempty")
		}
	})

	t.Run("invalid values", func(t *testing.T) {
		invalidInputs := []string{
			"\"5XiB\"", "\"invalid\"", "\"\"", "[]", "{}",
		}

		for _, invalid := range invalidInputs {
			t.Run(invalid, func(t *testing.T) {
				var b OptionalBytes
				err := json.Unmarshal([]byte(invalid), &b)
				assert.Error(t, err)
			})
		}
	})

	t.Run("panic on invalid stored value", func(t *testing.T) {
		// This tests that if somehow an invalid value gets stored
		// (bypassing UnmarshalJSON validation), WithDefault will panic
		invalidValue := "invalid-size"
		b := OptionalBytes{OptionalString{value: &invalidValue}}

		assert.Panics(t, func() {
			b.WithDefault(1024)
		}, "should panic on invalid stored value")
	})
}
