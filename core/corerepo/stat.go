package corerepo

import (
	"fmt"
	"math"

	context "context"

	"github.com/ipfs/go-ipfs/core"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	humanize "gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
)

// Stat wraps information about the objects stored on disk.
type Stat struct {
	RepoSize   uint64 // size in bytes
	StorageMax uint64 // size in bytes
	NumObjects uint64
	RepoPath   string
	Version    string
}

// NoLimit represents the value for unlimited storage
const NoLimit uint64 = math.MaxUint64

// RepoStat returns a *Stat object with all the fields set.
func RepoStat(ctx context.Context, n *core.IpfsNode) (*Stat, error) {
	sizeStat, err := RepoSize(ctx, n)
	if err != nil {
		return nil, err
	}

	allKeys, err := n.Blockstore.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}

	count := uint64(0)
	for range allKeys {
		count++
	}

	path, err := fsrepo.BestKnownPath()
	if err != nil {
		return nil, err
	}

	return &Stat{
		NumObjects: count,
		RepoSize:   sizeStat.RepoSize,
		StorageMax: sizeStat.StorageMax,
		RepoPath:   path,
		Version:    fmt.Sprintf("fs-repo@%d", fsrepo.RepoVersion),
	}, nil
}

// RepoSize returns a *Stat object with the RepoSize and StorageMax fields set.
func RepoSize(ctx context.Context, n *core.IpfsNode) (*Stat, error) {
	r := n.Repo

	cfg, err := r.Config()
	if err != nil {
		return nil, err
	}

	usage, err := r.GetStorageUsage()
	if err != nil {
		return nil, err
	}

	storageMax := NoLimit
	if cfg.Datastore.StorageMax != "" {
		storageMax, err = humanize.ParseBytes(cfg.Datastore.StorageMax)
		if err != nil {
			return nil, err
		}
	}

	return &Stat{
		RepoSize:   usage,
		StorageMax: storageMax,
	}, nil
}
