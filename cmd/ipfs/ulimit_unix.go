// +build linux darwin

package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

var ipfsFileDescNum = uint64(1024)

func init() {
	if val := os.Getenv("IPFS_FD_MAX"); val != "" {
		n, err := strconv.Atoi(val)
		if err != nil {
			log.Errorf("bad value for IPFS_FD_MAX: %s", err)
		} else {
			ipfsFileDescNum = uint64(n)
		}
	}
	fileDescriptorCheck = checkAndSetUlimit
}

func checkAndSetUlimit() error {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return fmt.Errorf("error getting rlimit: %s", err)
	}

	if rLimit.Cur < ipfsFileDescNum {
		if rLimit.Max < ipfsFileDescNum {
			rLimit.Max = ipfsFileDescNum
		}
		fmt.Printf("Adjusting current ulimit to %d.\n", ipfsFileDescNum)
		rLimit.Cur = ipfsFileDescNum
	}

	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return fmt.Errorf("error setting ulimit: %s", err)
	}

	return nil
}
