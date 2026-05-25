// Uses unix.Getxattr which is only available on Linux.
//go:build linux

package fuse

import "golang.org/x/sys/unix"

func getXattr(path, attr string) (string, error) {
	buf := make([]byte, 256)
	sz, err := unix.Getxattr(path, attr, buf)
	if err != nil {
		return "", err
	}
	return string(buf[:sz]), nil
}
