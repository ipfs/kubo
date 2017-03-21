package util

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestManageFdLimit(t *testing.T) {
	t.Log("Testing file descriptor count")
	if err := ManageFdLimit(); err != nil {
		t.Errorf("Cannot manage file descriptors")
	}

	if maxFds != uint64(2048) {
		t.Errorf("Maximum file descriptors default value changed")
	}
}

func TestManageInvalidNFds(t *testing.T) {
	t.Logf("Testing file descriptor invalidity")
	var err error
	if err = os.Unsetenv("IPFS_FD_MAX"); err != nil {
		t.Fatal("Cannot unset the IPFS_FD_MAX env variable")
	}

	rlimit := syscall.Rlimit{}
	if err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		t.Fatal("Cannot get the file descriptor count")
	}

	value := rlimit.Max + rlimit.Cur
	if err = os.Setenv("IPFS_FD_MAX", fmt.Sprintf("%d", value)); err != nil {
		t.Fatal("Cannot set the IPFS_FD_MAX env variable")
	}

	// call to check and set the maximum file descriptor from the env
	setMaxFds()

	if err = ManageFdLimit(); err == nil {
		t.Errorf("ManageFdLimit should return an error")
	} else if err != nil {
		flag := strings.Contains(err.Error(),
			"cannot set rlimit, IPFS_FD_MAX is larger than the hard limit")
		if !flag {
			t.Errorf("ManageFdLimit returned unexpected error")
		}
	}

	// unset all previous operations
	if err = os.Unsetenv("IPFS_FD_MAX"); err != nil {
		t.Fatal("Cannot unset the IPFS_FD_MAX env variable")
	}
}

func TestManageFdLimitWithEnvSet(t *testing.T) {
	t.Logf("Testing file descriptor manager with IPFS_FD_MAX set")
	var err error
	if err = os.Unsetenv("IPFS_FD_MAX"); err != nil {
		t.Fatal("Cannot unset the IPFS_FD_MAX env variable")
	}

	rlimit := syscall.Rlimit{}
	if err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		t.Fatal("Cannot get the file descriptor count")
	}

	value := rlimit.Max - rlimit.Cur + 1
	if err = os.Setenv("IPFS_FD_MAX", fmt.Sprintf("%d", value)); err != nil {
		t.Fatal("Cannot set the IPFS_FD_MAX env variable")
	}

	setMaxFds()
	if maxFds != value {
		t.Errorf("The maxfds is not set from IPFS_FD_MAX")
	}

	if err = ManageFdLimit(); err != nil {
		t.Errorf("Cannot manage file descriptor count")
	}

	// unset all previous operations
	if err = os.Unsetenv("IPFS_FD_MAX"); err != nil {
		t.Fatal("Cannot unset the IPFS_FD_MAX env variable")
	}
}
