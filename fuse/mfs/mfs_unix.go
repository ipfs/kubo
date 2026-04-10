// FUSE filesystem for the /mfs mount.
//
//go:build (linux || darwin || freebsd) && !nofuse

package mfs

import (
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/fuse/writable"
)

// NewFileSystem creates a new MFS FUSE root node.
func NewFileSystem(ipfs *core.IpfsNode, cfg config.Mounts) *writable.Dir {
	return writable.NewDir(ipfs.FilesRoot.GetDirectory(), &writable.Config{
		StoreMtime: cfg.StoreMtime.WithDefault(config.DefaultStoreMtime),
		StoreMode:  cfg.StoreMode.WithDefault(config.DefaultStoreMode),
		DAG:        ipfs.DAG,
		RepoPath:   ipfs.Repo.Path(),
	})
}
