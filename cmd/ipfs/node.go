package main

import (
	"context"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

type nodeBuilder struct {
	ctx context.Context
	repoPath string

}

func (b *nodeBuilder) buildNode() (*core.IpfsNode, error) {
	r, err := fsrepo.Open(b.repoPath)
	if err != nil { // repo is owned by the node
		return nil, err
	}

	// ok everything is good. set it on the invocation (for ownership)
	// and return it.
	n, err := core.NewNode(b.ctx, &core.BuildCfg{
		Repo: r,
	})
	if err != nil {
		return nil, err
	}

	return n, nil
}
