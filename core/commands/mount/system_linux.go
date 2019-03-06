package fusemount

import (
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
)

const (
	O_DIRECTORY = syscall.O_DIRECTORY
	O_NOFOLLOW  = syscall.O_NOFOLLOW
)

func (fs *FUSEIPFS) fuseFreeSize(fStatfs *fuse.Statfs_t, path string) error {
	sysStat := &syscall.Statfs_t{}
	if err := syscall.Statfs(path, sysStat); err != nil {
		return err
	}

	fStatfs.Fsid = uint64(sysStat.Fsid.X__val[0])<<32 | uint64(sysStat.Fsid.X__val[1])

	fStatfs.Bsize = uint64(sysStat.Bsize)
	fStatfs.Blocks = sysStat.Blocks
	fStatfs.Bfree = sysStat.Bfree
	fStatfs.Bavail = sysStat.Bavail
	fStatfs.Files = sysStat.Files
	fStatfs.Ffree = sysStat.Ffree
	fStatfs.Frsize = uint64(sysStat.Frsize)
	fStatfs.Flag = uint64(sysStat.Flags)
	fStatfs.Namemax = uint64(sysStat.Namelen)
	return nil
}
