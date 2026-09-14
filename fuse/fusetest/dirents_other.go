//go:build (darwin || freebsd) && !nofuse

package fusetest

import "testing"

// assertDirEntryInos compares getdents inode numbers with stat, which the
// linux build does. The syscall differs on every other platform and the FUSE
// suite runs on Linux, so there is nothing to check here.
func assertDirEntryInos(_ *testing.T, _ string) {}
