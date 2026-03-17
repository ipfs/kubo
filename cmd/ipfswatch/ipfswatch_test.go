//go:build !plan9

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsHidden(t *testing.T) {
	require.True(t, IsHidden("bar/.git"), "dirs beginning with . should be recognized as hidden")
	require.False(t, IsHidden("."), ". for current dir should not be considered hidden")
	require.False(t, IsHidden("bar/baz"), "normal dirs should not be hidden")
}
