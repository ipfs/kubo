package util

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("ulimit")

var (
	supportsFDManagement = false

	// getlimit returns the soft and hard limits of file descriptors counts
	getLimit func() (uint64, uint64, error)
	// set limit sets the soft and hard limits of file descriptors counts
	setLimit func(uint64, uint64) error
)

// minimum file descriptor limit before we complain
const minFds = 2048

// default max file descriptor limit.
const maxFds = 8192

// userMaxFDs returns the value of IPFS_FD_MAX
func userMaxFDs() uint64 {
	// check if the IPFS_FD_MAX is set up and if it does
	// not have a valid fds number notify the user
	if val := os.Getenv("IPFS_FD_MAX"); val != "" {
		fds, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Errorf("bad value for IPFS_FD_MAX: %s", err)
			return 0
		}
		return fds
	}
	return 0
}

// ManageFdLimit raise the current max file descriptor count
// of the process based on the IPFS_FD_MAX value
func ManageFdLimit() (changed bool, newLimit uint64, err error) {
	if !supportsFDManagement {
		return false, 0, nil
	}

	targetLimit := uint64(maxFds)
	userLimit := userMaxFDs()
	if userLimit > 0 {
		targetLimit = userLimit
	}

	soft, hard, err := getLimit()
	if err != nil {
		return false, 0, err
	}

	if targetLimit <= soft {
		return false, 0, nil
	}

	// the soft limit is the value that the kernel enforces for the
	// corresponding resource
	// the hard limit acts as a ceiling for the soft limit
	// an unprivileged process may only set it's soft limit to a
	// alue in the range from 0 up to the hard limit
	err = setLimit(targetLimit, targetLimit)
	switch err {
	case nil:
		newLimit = targetLimit
	case syscall.EPERM:
		// lower limit if necessary.
		if targetLimit > hard {
			targetLimit = hard
		}

		// the process does not have permission so we should only
		// set the soft value
		err = setLimit(targetLimit, hard)
		if err != nil {
			err = fmt.Errorf("error setting ulimit wihout hard limit: %s", err)
			break
		}
		newLimit = targetLimit

		// Warn on lowered limit.

		if newLimit < userLimit {
			err = fmt.Errorf(
				"failed to raise ulimit to IPFS_FD_MAX (%d): set to %d",
				userLimit,
				newLimit,
			)
			break
		}

		if userLimit == 0 && newLimit < minFds {
			err = fmt.Errorf(
				"failed to raise ulimit to minimum %d: set to %d",
				minFds,
				newLimit,
			)
			break
		}
	default:
		err = fmt.Errorf("error setting: ulimit: %s", err)
	}

	return newLimit > 0, newLimit, err
}
