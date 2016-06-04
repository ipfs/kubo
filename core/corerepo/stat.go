package corerepo

import (
	"github.com/ipfs/go-ipfs/core"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

type Stat struct {
	NumObjects uint64
	RepoSize   uint64 // size in bytes
	RepoPath   string
	Version    string
}

func RepoStat(n *core.IpfsNode, ctx context.Context) (*Stat, error) {
	r := n.Repo

	usage, err := r.GetStorageUsage()
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
		RepoSize:   usage,
		RepoPath:   path,
		Version:    "fs-repo@" + fsrepo.RepoVersion,
	}, nil
}
