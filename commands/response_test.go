package commands

import (
  "testing"
  "fmt"
)

type TestOutput struct {
  Foo, Bar string
  Baz int
}

func TestMarshalling(t *testing.T) {
  req := NewRequest()

  res := Response{
    req: req,
    Value: TestOutput{ "beep", "boop", 1337 },
  }

  _, err := res.Marshal()
  if err == nil {
    t.Error("Should have failed (no encoding type specified in request)")
  }

  req.SetOption(globalOptions[0], Json)
  bytes, err := res.Marshal()
  if err != nil {
    t.Error("Should have passed")
  }
  output := string(bytes)
  if output != "{\"Foo\":\"beep\",\"Bar\":\"boop\",\"Baz\":1337}" {
    t.Error("Incorrect JSON output")
  }

  res.SetError(fmt.Errorf("You broke something!"), Client)
  bytes, err = res.Marshal()
  if err != nil {
    t.Error("Should have passed")
  }
  output = string(bytes)
  if output != "{\"Message\":\"You broke something!\",\"Code\":1}" {
    t.Error("Incorrect JSON output")
  }
}
