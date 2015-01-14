// +build dragonfly freebsd netbsd openbsd

package fuseversion

import (
	"fmt"
	"runtime"
)

func getLocalFuseSystems() (*Systems, error) {
	return nil, fmt.Errorf(notImplYet, runtime.GOARCH)
}
