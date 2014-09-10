// +build !darwin

package syscallx

// This file just contains wrappers for platforms that already have
// the right stuff in stdlib.

import (
	"syscall"
)

func Getxattr(path string, attr string, dest []byte) (sz int, err error) {
	return syscall.Getxattr(path, attr, dest)
}

func Listxattr(path string, dest []byte) (sz int, err error) {
	return syscall.Listxattr(path, dest)
}

func Setxattr(path string, attr string, data []byte, flags int) (err error) {
	return syscall.Setxattr(path, attr, data, flags)
}

func Removexattr(path string, attr string) (err error) {
	return syscall.Removexattr(path, attr)
}
