package main

import (
	"runtime"
	"strings"
	"syscall"
)

func osxFuseCheck() error {
	// on OSX, check FUSE version.
	if runtime.GOOS != "darwin" {
		return nil
	}

	ov, err := syscall.Sysctl("osxfuse.version.number")
	if err != nil {
		return err
	}

	if strings.HasPrefix(ov, "2.7.") || strings.HasPrefix(ov, "2.8.") {
		return nil
	}

	return fmt.Errorf("osxfuse version %s not supported.\n%s\n%s", ov,
		"Older versions of osxfuse have kernel panic bugs; please upgrade!",
		"https://github.com/jbenet/go-ipfs/issues/177")
}
