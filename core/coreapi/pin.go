package coreapi

import (
	"context"
	"fmt"

	bserv "github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	merkledag "github.com/ipfs/go-merkledag"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
)

type PinAPI CoreAPI

func (api *PinAPI) Add(ctx context.Context, p coreiface.Path, opts ...caopts.PinAddOption) error {
	dagNode, err := api.core().ResolveNode(ctx, p)
	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	settings, err := caopts.PinAddOptions(opts...)
	if err != nil {
		return err
	}

	defer api.blockstore.PinLock().Unlock()

	err = api.pinning.Pin(ctx, dagNode, settings.Recursive)
	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	return api.pinning.Flush()
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

// Rm pin rm api
func (api *PinAPI) Rm(ctx context.Context, p coreiface.Path, opts ...caopts.PinRmOption) error {
	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return err
	}

	settings, err := caopts.PinRmOptions(opts...)
	if err != nil {
		return err
	}

	if err = api.pinning.Unpin(ctx, rp.Cid(), settings.Recursive); err != nil {
		return err
	}

	return api.pinning.Flush()
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

	defer api.blockstore.PinLock().Unlock()

	err = api.pinning.Update(ctx, fp.Cid(), tp.Cid(), settings.Unpin)
	if err != nil {
		return err
	}

	return api.pinning.Flush()
}

type pinStatus struct {
	cid      cid.Cid
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
	visited := make(map[cid.Cid]*pinStatus)
	bs := api.blockstore
	DAG := merkledag.NewDAGService(bserv.New(bs, offline.Exchange(bs)))
	getLinks := merkledag.GetLinksWithDAG(DAG)
	recPins := api.pinning.RecursiveKeys()

	var checkPin func(root cid.Cid) *pinStatus
	checkPin = func(root cid.Cid) *pinStatus {
		if status, ok := visited[root]; ok {
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := &pinStatus{ok: false, cid: root}
			status.badNodes = []coreiface.BadPinNode{&badNode{path: coreiface.IpldPath(root), err: err}}
			visited[root] = status
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

		visited[root] = status
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

	keys := make(map[cid.Cid]*pinInfo)

	AddToResultKeys := func(keyList []cid.Cid, typeStr string) {
		for _, c := range keyList {
			keys[c] = &pinInfo{
				pinType: typeStr,
				path:    coreiface.IpldPath(c),
			}
		}
	}

	if typeStr == "direct" || typeStr == "all" {
		AddToResultKeys(api.pinning.DirectKeys(), "direct")
	}
	if typeStr == "indirect" || typeStr == "all" {
		set := cid.NewSet()
		for _, k := range api.pinning.RecursiveKeys() {
			err := merkledag.EnumerateChildren(ctx, merkledag.GetLinksWithDAG(api.dag), k, set.Visit)
			if err != nil {
				return nil, err
			}
		}
		AddToResultKeys(set.Keys(), "indirect")
	}
	if typeStr == "recursive" || typeStr == "all" {
		AddToResultKeys(api.pinning.RecursiveKeys(), "recursive")
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
