// macFUSE/OSXFUSE availability check. Darwin only.
//go:build darwin && !nofuse

package node

import (
	"fmt"
	"os"

	core "github.com/ipfs/kubo/core"
)

func init() {
	platformFuseChecks = darwinFuseCheck
}

// macFUSE mount helper paths, checked in the same order as go-fuse.
var macFUSEPaths = []string{
	"/Library/Filesystems/macfuse.fs/Contents/Resources/mount_macfuse",
	"/Library/Filesystems/osxfuse.fs/Contents/Resources/mount_osxfuse",
}

func darwinFuseCheck(_ *core.IpfsNode) error {
	for _, p := range macFUSEPaths {
		if _, err := os.Stat(p); err == nil {
			return nil
		}
	}
	return fmt.Errorf(`macFUSE not found.

macFUSE is required to mount FUSE filesystems on macOS.
Install it from https://osxfuse.github.io/ or via Homebrew:

    brew install macfuse
`)
}
