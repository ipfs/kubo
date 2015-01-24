// +build windows

package watch

import (
	"os"
)

const permissionDeniedRetryCount int = 5

func permissionErrorRetry(err error, retry *int) bool {
	if os.IsPermission(err) && *retry < permissionDeniedRetryCount {
		// While pooling a file that does not exist yet, but will be created by another process we can get Permission Denied
		(*retry)++
		return true
	}
	return false
}
