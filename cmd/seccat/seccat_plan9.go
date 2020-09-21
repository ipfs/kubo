package main

import (
	"os"
	"syscall"
)

var notifySignals = []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM}
