# go-fuse-version

Simple package to get the user's FUSE libraries information.

- Godoc: https://godoc.org/github.com/jbenet/go-fuse-version

**Warning** Currently only supports OSXFUSE. if you want more, add them, it's really trivial now.

## Example

```Go
package main

import (
  "fmt"
  "os"

  fuseversion "github.com/jbenet/go-fuse-version"
)

func main() {
  sys, err := fuseversion.LocalFuseSystems()
  if err != nil {
    fmt.Fprintf(os.Stderr, "%s\n", err)
    os.Exit(1)
  }

  fmt.Printf("FuseVersion, AgentVersion, Agent\n")
  for _, s := range *sys {
    fmt.Printf("%s, %s, %s\n", s.FuseVersion, s.AgentVersion, s.AgentName)
  }
}
```

## fuse-print

If you dont use Go, you can also install the example as the silly util `fuse-version`:

```
> go get github.com/jbenet/go-fuse-version/fuse-version
> go install github.com/jbenet/go-fuse-version/fuse-version
> fuse-version
FuseVersion, AgentVersion, Agent
27, 2.7.2, OSXFUSE
```
