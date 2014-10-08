package commands

import "testing"

func TestOptionValidation(t *testing.T) {
  cmd := Command{
    Options: []Option{
      Option{ []string{ "b", "beep" }, Int },
      Option{ []string{ "B", "boop" }, String },
    },
    f: func(req *Request) (interface{}, error) {
      return nil, nil
    },
  }

  req := NewRequest()
  req.options["foo"] = 5
  _, err := cmd.Call(nil, req)
  if err == nil {
    t.Error("Should have failed (unrecognized command)")
  }

  req = NewRequest()
  req.options["beep"] = 5
  req.options["b"] = 10
  _, err = cmd.Call(nil, req)
  if err == nil {
    t.Error("Should have failed (duplicate options)")
  }

  req = NewRequest()
  req.options["beep"] = "foo"
  _, err = cmd.Call(nil, req)
  if err == nil {
    t.Error("Should have failed (incorrect type)")
  }

  req = NewRequest()
  req.options["beep"] = 5
  _, err = cmd.Call(nil, req)
  if err != nil {
    t.Error("Should have passed")
  }

  req = NewRequest()
  req.options["beep"] = 5
  req.options["boop"] = "test"
  _, err = cmd.Call(nil, req)
  if err != nil {
    t.Error("Should have passed")
  }

  req = NewRequest()
  req.options["b"] = 5
  req.options["B"] = "test"
  _, err = cmd.Call(nil, req)
  if err != nil {
    t.Error("Should have passed")
  }
}
