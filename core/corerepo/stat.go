package corerepo

import (
	"fmt"

	context "context"
	"github.com/ipfs/go-ipfs/core"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
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
		Version:    fmt.Sprintf("fs-repo@%d", fsrepo.RepoVersion),
	}, nil
}
