package fuse

import (
	"runtime"
)

func stack() string {
	buf := make([]byte, 1024)
	return string(buf[:runtime.Stack(buf, false)])
}

func nop(msg interface{}) {}

var Debug func(msg interface{}) = nop
