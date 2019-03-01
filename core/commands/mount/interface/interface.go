package mountinterface

import (
	"github.com/billziss-gh/cgofuse/fuse"
)

//TODO: docs; this is only necessary to prevent cyclical imports in core; necessary for daemon to retain knowledge and control of mountpoint
type Interface interface {
	fuse.FileSystemInterface
	IsActive() bool
	Where() string
	Close() error
}
