package cli

import (
  //"fmt"
  "testing"
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
}
