//go:build !plan9
// +build !plan9

package main

import (
	"testing"

	"github.com/ipfs/kubo/thirdparty/assert"
)

func TestIsHidden(t *testing.T) {
	assert.True(IsHidden("bar/.git"), t, "dirs beginning with . should be recognized as hidden")
	assert.False(IsHidden("."), t, ". for current dir should not be considered hidden")
	assert.False(IsHidden("bar/baz"), t, "normal dirs should not be hidden")
}
