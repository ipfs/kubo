package config

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	out, err := json.Marshal(Duration(time.Second))
	if err != nil {
		t.Fatal(err)

	}
	expected := "\"1s\""
	if string(out) != expected {
		t.Fatalf("expected %s, got %s", expected, string(out))
	}
	var d Duration
	err = json.Unmarshal(out, &d)
	if err != nil {
		t.Fatal(err)
	}
	if time.Duration(d) != time.Second {
		t.Fatal("expected a second")
	}
	type Foo struct {
		D Duration `json:",omitempty"`
	}
	out, err = json.Marshal(new(Foo))
	if err != nil {
		t.Fatal(err)
	}
	expected = "{}"
	if string(out) != expected {
		t.Fatal("expected omitempty to omit the duration")
	}
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
