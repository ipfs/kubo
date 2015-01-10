package fuseversion

// #cgo CFLAGS: -I /usr/local/include/osxfuse/ -D_FILE_OFFSET_BITS=64 -DFUSE_USE_VERSION=25
// #cgo LDFLAGS: /usr/local/lib/libosxfuse.dylib
//
// #include <fuse/fuse.h>
// #include <fuse/fuse_common.h>
// #include <fuse/fuse_darwin.h>
import "C"
import "fmt"

func getLocalFuseSystems() (*Systems, error) {
	sys := Systems{}
	sys["OSXFUSE"] = getOSXFUSE()
	return &sys, nil
}

func getOSXFUSE() FuseSystem {
	return FuseSystem{
		FuseVersion:  fmt.Sprintf("%d", int(C.fuse_version())),
		AgentName:    "OSXFUSE",
		AgentVersion: C.GoString(C.osxfuse_version()),
	}
}
