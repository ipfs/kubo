package config

import (
	"encoding/json"
	"testing"
)

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
