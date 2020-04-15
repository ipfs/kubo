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
