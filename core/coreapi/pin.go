package coreapi

import (
	"context"
	"fmt"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	merkledag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	bserv "gx/ipfs/QmTZZrpd9o4vpYr9TEADW2EoJ9fzUtAgpXqjxZHbKR2T15/go-blockservice"

	offline "gx/ipfs/QmVozMmsgK2PYyaHQsrcWLBYigb1m6mW8YhCBG2Cb4Uxq9/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
)

type PinAPI CoreAPI

func (api *PinAPI) Add(ctx context.Context, p coreiface.Path, opts ...caopts.PinAddOption) error {
	settings, err := caopts.PinAddOptions(opts...)
	if err != nil {
		return err
	}

	defer api.node.Blockstore.PinLock().Unlock()

	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return err
	}

	_, err = corerepo.Pin(api.node, ctx, []string{rp.Cid().String()}, settings.Recursive)
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

	return api.pinLsAll(settings.Type, ctx)
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

	fp, err := api.core().ResolvePath(ctx, from)
	if err != nil {
		return err
	}

	tp, err := api.core().ResolvePath(ctx, to)
	if err != nil {
		return err
	}

	return api.node.Pinning.Update(ctx, fp.Cid(), tp.Cid(), settings.Unpin)
}

type pinStatus struct {
	cid      *cid.Cid
	ok       bool
	badNodes []coreiface.BadPinNode
}

// BadNode is used in PinVerifyRes
type badNode struct {
	path coreiface.ResolvedPath
	err  error
}

func (s *pinStatus) Ok() bool {
	return s.ok
}

func (s *pinStatus) BadNodes() []coreiface.BadPinNode {
	return s.badNodes
}

func (n *badNode) Path() coreiface.ResolvedPath {
	return n.path
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
			status.badNodes = []coreiface.BadPinNode{&badNode{path: coreiface.IpldPath(root), err: err}}
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
	path    coreiface.ResolvedPath
}

func (p *pinInfo) Path() coreiface.ResolvedPath {
	return p.path
}

func (p *pinInfo) Type() string {
	return p.pinType
}

func (api *PinAPI) pinLsAll(typeStr string, ctx context.Context) ([]coreiface.Pin, error) {

	keys := make(map[string]*pinInfo)

	AddToResultKeys := func(keyList []*cid.Cid, typeStr string) {
		for _, c := range keyList {
			keys[c.String()] = &pinInfo{
				pinType: typeStr,
				path:    coreiface.IpldPath(c),
			}
		}
	}

	if typeStr == "direct" || typeStr == "all" {
		AddToResultKeys(api.node.Pinning.DirectKeys(), "direct")
	}
	if typeStr == "indirect" || typeStr == "all" {
		set := cid.NewSet()
		for _, k := range api.node.Pinning.RecursiveKeys() {
			err := merkledag.EnumerateChildren(ctx, merkledag.GetLinksWithDAG(api.node.DAG), k, set.Visit)
			if err != nil {
				return nil, err
			}
		}
		AddToResultKeys(set.Keys(), "indirect")
	}
	if typeStr == "recursive" || typeStr == "all" {
		AddToResultKeys(api.node.Pinning.RecursiveKeys(), "recursive")
	}

	out := make([]coreiface.Pin, 0, len(keys))
	for _, v := range keys {
		out = append(out, v)
	}

	return out, nil
}

func (api *PinAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
