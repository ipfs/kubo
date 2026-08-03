// FUSE filesystem for the /mfs mount.
//
//go:build (linux || darwin || freebsd) && !nofuse

package mfs

import (
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
	"github.com/ipfs/kubo/fuse/writable"
)

// NewFileSystem creates a new MFS FUSE root node.
func NewFileSystem(ipfs *core.IpfsNode, mounts config.Mounts, imp config.Import) *writable.Dir {
	return writable.NewDir(ipfs.FilesRoot.GetDirectory(), &writable.Config{
		StoreMtime: mounts.StoreMtime.WithDefault(config.DefaultStoreMtime),
		StoreMode:  mounts.StoreMode.WithDefault(config.DefaultStoreMode),
		DAG:        ipfs.DAG,
		RepoPath:   ipfs.Repo.Path(),
		Blksize:    fusemnt.BlksizeFromChunker(imp.UnixFSChunker.WithDefault(config.DefaultUnixFSChunker)),
		// This mount writes ipfs.FilesRoot, the same MFS root the `ipfs files`
		// commands use, so guard its writes with the pin lock too. GC already
		// protects that root's blocks via corerepo.BestEffortRoots.
		GCLocker: ipfs.Blockstore,
		// Long-lived write descriptors bind to the node context so their
		// writes are cancelled on shutdown instead of blocking forever.
		MountCtx: ipfs.Context(),
	})
}
