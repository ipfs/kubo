package corerepo

import (
	"fmt"
	"math"

	context "context"

	"github.com/ipfs/go-ipfs/core"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	humanize "gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
)

// SizeStat wraps information about the repository size and its limit.
type SizeStat struct {
	RepoSize   uint64 // size in bytes
	StorageMax uint64 // size in bytes
}

// Stat wraps information about the objects stored on disk.
type Stat struct {
	SizeStat
	NumObjects uint64
	RepoPath   string
	Version    string
}

// NoLimit represents the value for unlimited storage
const NoLimit uint64 = math.MaxUint64

// RepoStat returns a *Stat object with all the fields set.
func RepoStat(ctx context.Context, n *core.IpfsNode) (Stat, error) {
	sizeStat, err := RepoSize(ctx, n)
	if err != nil {
		return Stat{}, err
	}

	allKeys, err := n.Blockstore.AllKeysChan(ctx)
	if err != nil {
		return Stat{}, err
	}

	count := uint64(0)
	for range allKeys {
		count++
	}

	path, err := fsrepo.BestKnownPath()
	if err != nil {
		return Stat{}, err
	}

	return Stat{
		SizeStat: SizeStat{
			RepoSize:   sizeStat.RepoSize,
			StorageMax: sizeStat.StorageMax,
		},
		NumObjects: count,
		RepoPath:   path,
		Version:    fmt.Sprintf("fs-repo@%d", fsrepo.RepoVersion),
	}, nil
}

// RepoSize returns a *Stat object with the RepoSize and StorageMax fields set.
func RepoSize(ctx context.Context, n *core.IpfsNode) (SizeStat, error) {
	r := n.Repo

	cfg, err := r.Config()
	if err != nil {
		return SizeStat{}, err
	}

	usage, err := r.GetStorageUsage()
	if err != nil {
		return SizeStat{}, err
	}

	storageMax := NoLimit
	if cfg.Datastore.StorageMax != "" {
		storageMax, err = humanize.ParseBytes(cfg.Datastore.StorageMax)
		if err != nil {
			return SizeStat{}, err
		}
	}

	return SizeStat{
		RepoSize:   usage,
		StorageMax: storageMax,
	}, nil
}
