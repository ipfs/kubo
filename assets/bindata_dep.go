//+build never,!never

package assets

import (
	// Make sure go mod tracks these deps but avoid including them in the
	// actual build.
	_ "github.com/go-bindata/go-bindata/v3"
)
