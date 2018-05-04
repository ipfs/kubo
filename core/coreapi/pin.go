package coreapi

import (
	"context"
	"fmt"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"

	offline "gx/ipfs/QmYk9mQ4iByLLFzZPGWMnjJof3DQ3QneFFR6ZtNAXd8UvS/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

type PinAPI CoreAPI

func (api *PinAPI) Add(ctx context.Context, p coreiface.Path, opts ...caopts.PinAddOption) error {
	settings, err := caopts.PinAddOptions(opts...)
	if err != nil {
		return err
	}

	defer api.node.Blockstore.PinLock().Unlock()

	_, err = corerepo.Pin(api.node, ctx, []string{p.String()}, settings.Recursive)
	if err != nil {
		return err
	}

	return nil
}

func (api *PinAPI) Ls(ctx context.Context, opts ...caopts.PinLsOption) ([]coreiface.Pin, error) {
	settings, err := caopts.PinLsOptions(opts...)
	if err != nil {
		return nil, err
	}

	switch settings.Type {
	case "all", "direct", "indirect", "recursive":
	default:
		return nil, fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", settings.Type)
	}

	return pinLsAll(settings.Type, ctx, api.node.Pinning, api.node.DAG)
}

func (api *PinAPI) Rm(ctx context.Context, p coreiface.Path) error {
	_, err := corerepo.Unpin(api.node, ctx, []string{p.String()}, true)
	if err != nil {
		return err
	}

	return nil
}

func (api *PinAPI) Update(ctx context.Context, from coreiface.Path, to coreiface.Path, opts ...caopts.PinUpdateOption) error {
	settings, err := caopts.PinUpdateOptions(opts...)
	if err != nil {
		return err
	}

	return api.node.Pinning.Update(ctx, from.Cid(), to.Cid(), settings.Unpin)
}

type pinStatus struct {
	cid      *cid.Cid
	ok       bool
	badNodes []coreiface.BadPinNode
}

// BadNode is used in PinVerifyRes
type badNode struct {
	cid *cid.Cid
	err error
}

func (s *pinStatus) Ok() bool {
	return s.ok
}

func (s *pinStatus) BadNodes() []coreiface.BadPinNode {
	return s.badNodes
}

func (n *badNode) Path() coreiface.Path {
	return ParseCid(n.cid)
}

func (n *badNode) Err() error {
	return n.err
}

func (api *PinAPI) Verify(ctx context.Context) (<-chan coreiface.PinStatus, error) {
	visited := make(map[string]*pinStatus)
	bs := api.node.Blocks.Blockstore()
	DAG := merkledag.NewDAGService(bserv.New(bs, offline.Exchange(bs)))
	getLinks := merkledag.GetLinksWithDAG(DAG)
	recPins := api.node.Pinning.RecursiveKeys()

	var checkPin func(root *cid.Cid) *pinStatus
	checkPin = func(root *cid.Cid) *pinStatus {
		key := root.String()
		if status, ok := visited[key]; ok {
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := &pinStatus{ok: false, cid: root}
			status.badNodes = []coreiface.BadPinNode{&badNode{cid: root, err: err}}
			visited[key] = status
			return status
		}

		status := &pinStatus{ok: true, cid: root}
		for _, lnk := range links {
			res := checkPin(lnk.Cid)
			if !res.ok {
				status.ok = false
				status.badNodes = append(status.badNodes, res.badNodes...)
			}
		}

		visited[key] = status
		return status
	}

	out := make(chan coreiface.PinStatus)
	go func() {
		defer close(out)
		for _, c := range recPins {
			out <- checkPin(c)
		}
	}()

	return out, nil
}

type pinInfo struct {
	pinType string
	object  *cid.Cid
}

func (p *pinInfo) Path() coreiface.Path {
	return ParseCid(p.object)
}

func (p *pinInfo) Type() string {
	return p.pinType
}

func pinLsAll(typeStr string, ctx context.Context, pinning pin.Pinner, dag ipld.DAGService) ([]coreiface.Pin, error) {

	keys := make(map[string]*pinInfo)

	AddToResultKeys := func(keyList []*cid.Cid, typeStr string) {
		for _, c := range keyList {
			keys[c.String()] = &pinInfo{
				pinType: typeStr,
				object:  c,
			}
		}
	}

	if typeStr == "direct" || typeStr == "all" {
		AddToResultKeys(pinning.DirectKeys(), "direct")
	}
	if typeStr == "indirect" || typeStr == "all" {
		set := cid.NewSet()
		for _, k := range pinning.RecursiveKeys() {
			err := merkledag.EnumerateChildren(ctx, merkledag.GetLinksWithDAG(dag), k, set.Visit)
			if err != nil {
				return nil, err
			}
		}
		AddToResultKeys(set.Keys(), "indirect")
	}
	if typeStr == "recursive" || typeStr == "all" {
		AddToResultKeys(pinning.RecursiveKeys(), "recursive")
	}

	out := make([]coreiface.Pin, 0, len(keys))
	for _, v := range keys {
		out = append(out, v)
	}

	return out, nil
}
