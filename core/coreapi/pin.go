package coreapi

import (
	"context"
	"errors"
	"fmt"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/thirdparty/recpinset"

	offline "gx/ipfs/QmPf114DXfa6TqGKYhBGR7EtXRho4rCJgwyA1xkuMY5vwF/go-ipfs-exchange-offline"
	ipld "gx/ipfs/QmWi2BYBL5gJ3CiAiQchg6rn1A8iBsrWy51EYxvHVjFvLb/go-ipld-format"
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
)

type PinAPI CoreAPI

func (api *PinAPI) Add(ctx context.Context, p coreiface.Path, opts ...caopts.PinAddOption) error {
	settings, err := caopts.PinAddOptions(opts...)
	if err != nil {
		return err
	}

	defer api.node.Blockstore.PinLock().Unlock()

	if !settings.Recursive {
		settings.MaxDepth = 0
	}

	if settings.Recursive && settings.MaxDepth == 0 {
		return errors.New("bad MaxDepth=0. Pin is Recursive. Use non-recursive pin")
	}

	if settings.MaxDepth < 0 {
		settings.MaxDepth = -1
	}

	_, err = corerepo.Pin(api.node, ctx, []string{p.String()}, settings.MaxDepth)
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
	statuses := make(map[string]*pinStatus)
	visited := recpinset.New()
	bs := api.node.Blocks.Blockstore()
	DAG := merkledag.NewDAGService(bserv.New(bs, offline.Exchange(bs)))
	getLinks := merkledag.GetLinksWithDAG(DAG)
	recPins := api.node.Pinning.RecursivePins()

	var checkPinMaxDepth func(root *cid.Cid, maxDepth int) *pinStatus
	checkPinMaxDepth = func(root *cid.Cid, maxDepth int) *pinStatus {
		key := root.String()
		// it was visited already, return last status
		if !visited.Visit(root, maxDepth) {
			return statuses[key]
		}

		if maxDepth == 0 {
			return &pinStatus{ok: true, cid: root}
		}

		if maxDepth > 0 {
			maxDepth--
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := &pinStatus{ok: false, cid: root}
			status.badNodes = []coreiface.BadPinNode{&badNode{cid: root, err: err}}
			statuses[key] = status
			return status
		}

		status := &pinStatus{ok: true, cid: root}
		for _, lnk := range links {
			res := checkPinMaxDepth(lnk.Cid, maxDepth)
			if !res.ok {
				status.ok = false
				status.badNodes = append(status.badNodes, res.badNodes...)
			}
		}

		statuses[key] = status
		return status
	}

	out := make(chan coreiface.PinStatus)
	go func() {
		defer close(out)
		for _, c := range recPins {
			out <- checkPinMaxDepth(c.Cid, c.MaxDepth)
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
		set := recpinset.New()
		for _, recPin := range pinning.RecursivePins() {
			err := merkledag.EnumerateChildrenMaxDepth(
				ctx,
				merkledag.GetLinksWithDAG(dag),
				recPin.Cid,
				recPin.MaxDepth,
				set.Visit,
			)
			if err != nil {
				return nil, err
			}
		}
		AddToResultKeys(set.Keys(), "indirect")
	}
	if typeStr == "recursive" || typeStr == "all" {
		for _, recPin := range pinning.RecursivePins() {
			mode, _ := pin.ModeToString(pin.MaxDepthToMode(recPin.MaxDepth))
			keys[recPin.Cid.String()] = &pinInfo{
				pinType: mode,
				object:  recPin.Cid,
			}
		}
	}

	out := make([]coreiface.Pin, 0, len(keys))
	for _, v := range keys {
		out = append(out, v)
	}

	return out, nil
}
