# go-cienv - CI system env vars

[![travisbadge](https://travis-ci.org/jbenet/go-cienv.svg)](https://travis-ci.org/jbenet/go-cienv)

Simple packages to use your CI system's environment variables.

- godoc: https://godoc.org/github.com/jbenet/go-cienv

Example:

```go
import (
  "testing"

  travis "github.com/jbenet/go-cienv/travis"
)

func TestFoo(t *testing.T) {
  if travis.IsRunning() {
    t.Skip("this test cant run on travis")
  }
}
```
