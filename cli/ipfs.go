package main

import (
  "fmt"
  "os"
)

func main() {
  err := CmdIpfs.Dispatch(os.Args[1:])
  if err != nil {
    if len(err.Error()) > 0 {
      fmt.Fprintf(os.Stderr, "%v\n", err)
    }
    os.Exit(1)
  }
  return
}
