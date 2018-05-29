package util

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
)

var log = logging.Logger("ulimit")

var (
	supportsFDManagement = false

	// getlimit returns the soft and hard limits of file descriptors counts
	getLimit func() (int64, int64, error)
	// set limit sets the soft and hard limits of file descriptors counts
	setLimit func(int64, int64) error
)

// maxFds is the maximum number of file descriptors that go-ipfs
// can use. The default value is 1024. This can be overwritten by the
// IPFS_FD_MAX env variable
var maxFds = uint64(2048)

// setMaxFds sets the maxFds value from IPFS_FD_MAX
// env variable if it's present on the system
func setMaxFds() {
	// check if the IPFS_FD_MAX is set up and if it does
	// not have a valid fds number notify the user
	if val := os.Getenv("IPFS_FD_MAX"); val != "" {

		fds, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Errorf("bad value for IPFS_FD_MAX: %s", err)
			return
		}

		maxFds = fds
	}
}

// ManageFdLimit raise the current max file descriptor count
// of the process based on the IPFS_FD_MAX value
func ManageFdLimit() error {
	if !supportsFDManagement {
		return nil
	}

	setMaxFds()
	soft, hard, err := getLimit()
	if err != nil {
		return err
	}

	max := int64(maxFds)

	if max <= soft {
		return nil
	}

	// the soft limit is the value that the kernel enforces for the
	// corresponding resource
	// the hard limit acts as a ceiling for the soft limit
	// an unprivileged process may only set it's soft limit to a
	// alue in the range from 0 up to the hard limit
	if err = setLimit(max, max); err != nil {
		if err != syscall.EPERM {
			return fmt.Errorf("error setting: ulimit: %s", err)
		}

		// the process does not have permission so we should only
		// set the soft value
		if max > hard {
			return errors.New(
				"cannot set rlimit, IPFS_FD_MAX is larger than the hard limit",
			)
		}

		if err = setLimit(max, hard); err != nil {
			return fmt.Errorf("error setting ulimit wihout hard limit: %s", err)
		}
	}

	fmt.Printf("Successfully raised file descriptor limit to %d.\n", max)

	return nil
}
