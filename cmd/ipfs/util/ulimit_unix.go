// +build darwin linux netbsd openbsd

package util

import (
	unix "gx/ipfs/QmVGjyM9i2msKvLXwh9VosCTgP4mL91kC7hDmqnwTTx6Hu/sys/unix"
)

func init() {
	supportsFDManagement = true
	getLimit = unixGetLimit
	setLimit = unixSetLimit
}

func unixGetLimit() (int64, int64, error) {
	rlimit := unix.Rlimit{}
	err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit)
	return int64(rlimit.Cur), int64(rlimit.Max), err
}

func unixSetLimit(soft int64, max int64) error {
	rlimit := unix.Rlimit{
		Cur: uint64(soft),
		Max: uint64(max),
	}
	return unix.Setrlimit(unix.RLIMIT_NOFILE, &rlimit)
}
