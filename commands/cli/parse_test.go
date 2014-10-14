package cli

import (
  //"fmt"
  "testing"

  "github.com/jbenet/go-ipfs/commands"
)

func TestOptionParsing(t *testing.T) {
  opts, input, err := options([]string{ "test", "--beep", "--boop=\"5", "lol\"", "test2", "-cV" }, nil)
  /*for k, v := range opts {
    fmt.Printf("%s: %s\n", k, v)
  }
  fmt.Printf("%s\n", input)*/
  if err != nil {
    t.Error("Should have passed")
  }
  if len(opts) != 4 || opts["c"] != "" || opts["V"] != "" || opts["beep"] != "" || opts["boop"] != "5 lol" {
    t.Error("Returned options were defferent than expected: %v", opts)
  }
  if len(input) != 2 || input[0] != "test" || input[1] != "test2" {
    t.Error("Returned input was different than expected: %v", input)
  }

  cmd := &commands.Command{}
  cmd.Register("test", &commands.Command{})
  path, args, err := path([]string{ "test", "beep", "boop" }, cmd)
  if err != nil {
    t.Error("Should have passed")
  }
  if len(path) != 1 || path[0] != "test" {
    t.Error("Returned path was defferent than expected: %v", path)
  }
  if len(args) != 2 || args[0] != "beep" || args[1] != "boop" {
    t.Error("Returned args were different than expected: %v", args)
  }
}
