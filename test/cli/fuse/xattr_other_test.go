// Stub that skips xattr tests on non-Linux platforms.
//go:build !linux

package fuse

import (
	"fmt"
	"runtime"
)

func getXattr(_, _ string) (string, error) {
	return "", fmt.Errorf("xattr not supported on %s", runtime.GOOS)
}
