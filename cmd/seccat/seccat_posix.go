// +build !plan9

package main

import (
	"os"
	"syscall"
)

var notifySignals = []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}
