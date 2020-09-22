package coreapi

import (
	"context"
	"fmt"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	pin "github.com/ipfs/go-ipfs-pinner"
	"github.com/ipfs/go-merkledag"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

type PinAPI CoreAPI

func (api *PinAPI) Add(ctx context.Context, p path.Path, opts ...caopts.PinAddOption) error {
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

	if err := api.provider.Provide(dagNode.Cid()); err != nil {
		return err
	}

	return api.pinning.Flush(ctx)
}

func (api *PinAPI) Ls(ctx context.Context, opts ...caopts.PinLsOption) (<-chan coreiface.Pin, error) {
	settings, err := caopts.PinLsOptions(opts...)
	if err != nil {
		return nil, err
	}

	switch settings.Type {
	case "all", "direct", "indirect", "recursive":
	default:
		return nil, fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", settings.Type)
	}

	return api.pinLsAll(ctx, settings.Type), nil
}

func (api *PinAPI) IsPinned(ctx context.Context, p path.Path, opts ...caopts.PinIsPinnedOption) (string, bool, error) {
	resolved, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return "", false, fmt.Errorf("error resolving path: %s", err)
	}

	settings, err := caopts.PinIsPinnedOptions(opts...)
	if err != nil {
		return "", false, err
	}

	mode, ok := pin.StringToMode(settings.WithType)
	if !ok {
		return "", false, fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", settings.WithType)
	}

	return api.pinning.IsPinnedWithType(ctx, resolved.Cid(), mode)
}

// Rm pin rm api
func (api *PinAPI) Rm(ctx context.Context, p path.Path, opts ...caopts.PinRmOption) error {
	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return err
	}

	settings, err := caopts.PinRmOptions(opts...)
	if err != nil {
		return err
	}

	// Note: after unpin the pin sets are flushed to the blockstore, so we need
	// to take a lock to prevent a concurrent garbage collection
	defer api.blockstore.PinLock().Unlock()

	if err = api.pinning.Unpin(ctx, rp.Cid(), settings.Recursive); err != nil {
		return err
	}

	return api.pinning.Flush(ctx)
}

func (api *PinAPI) Update(ctx context.Context, from path.Path, to path.Path, opts ...caopts.PinUpdateOption) error {
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

	return api.pinning.Flush(ctx)
}

type pinStatus struct {
	cid      cid.Cid
	ok       bool
	badNodes []coreiface.BadPinNode
}

// BadNode is used in PinVerifyRes
type badNode struct {
	path path.Resolved
	err  error
}

func (s *pinStatus) Ok() bool {
	return s.ok
}

func (s *pinStatus) BadNodes() []coreiface.BadPinNode {
	return s.badNodes
}

func (n *badNode) Path() path.Resolved {
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
	recPins, err := api.pinning.RecursiveKeys(ctx)
	if err != nil {
		return nil, err
	}

	var checkPin func(root cid.Cid) *pinStatus
	checkPin = func(root cid.Cid) *pinStatus {
		if status, ok := visited[root]; ok {
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := &pinStatus{ok: false, cid: root}
			status.badNodes = []coreiface.BadPinNode{&badNode{path: path.IpldPath(root), err: err}}
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
	path    path.Resolved
	err     error
}

func (p *pinInfo) Path() path.Resolved {
	return p.path
}

func (p *pinInfo) Type() string {
	return p.pinType
}

func (p *pinInfo) Err() error {
	return p.err
}

// pinLsAll is an internal function for returning a list of pins
func (api *PinAPI) pinLsAll(ctx context.Context, typeStr string) <-chan coreiface.Pin {
	out := make(chan coreiface.Pin)

	keys := cid.NewSet()

	AddToResultKeys := func(keyList []cid.Cid, typeStr string) error {
		for _, c := range keyList {
			if keys.Visit(c) {
				select {
				case out <- &pinInfo{
					pinType: typeStr,
					path:    path.IpldPath(c),
				}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		return nil
	}

	VisitKeys := func(keyList []cid.Cid) {
		for _, c := range keyList {
			keys.Visit(c)
		}
	}

	go func() {
		defer close(out)

		if typeStr == "recursive" || typeStr == "all" {
			rkeys, err := api.pinning.RecursiveKeys(ctx)
			if err != nil {
				out <- &pinInfo{err: err}
				return
			}
			if err := AddToResultKeys(rkeys, "recursive"); err != nil {
				out <- &pinInfo{err: err}
				return
			}
		}
		if typeStr == "direct" || typeStr == "all" {
			dkeys, err := api.pinning.DirectKeys(ctx)
			if err != nil {
				out <- &pinInfo{err: err}
				return
			}
			if err := AddToResultKeys(dkeys, "direct"); err != nil {
				out <- &pinInfo{err: err}
				return
			}
		}
		if typeStr == "all" {
			set := cid.NewSet()
			rkeys, err := api.pinning.RecursiveKeys(ctx)
			if err != nil {
				out <- &pinInfo{err: err}
				return
			}
			for _, k := range rkeys {
				err := merkledag.Walk(
					ctx, merkledag.GetLinksWithDAG(api.dag), k,
					set.Visit,
					merkledag.SkipRoot(), merkledag.Concurrent(),
				)
				if err != nil {
					out <- &pinInfo{err: err}
					return
				}
			}
			if err := AddToResultKeys(set.Keys(), "indirect"); err != nil {
				out <- &pinInfo{err: err}
				return
			}
		}
		if typeStr == "indirect" {
			// We need to first visit the direct pins that have priority
			// without emitting them

			dkeys, err := api.pinning.DirectKeys(ctx)
			if err != nil {
				out <- &pinInfo{err: err}
				return
			}
			VisitKeys(dkeys)

			rkeys, err := api.pinning.RecursiveKeys(ctx)
			if err != nil {
				out <- &pinInfo{err: err}
				return
			}
			VisitKeys(rkeys)

			set := cid.NewSet()
			for _, k := range rkeys {
				err := merkledag.Walk(
					ctx, merkledag.GetLinksWithDAG(api.dag), k,
					set.Visit,
					merkledag.SkipRoot(), merkledag.Concurrent(),
				)
				if err != nil {
					out <- &pinInfo{err: err}
					return
				}
			}
			if err := AddToResultKeys(set.Keys(), "indirect"); err != nil {
				out <- &pinInfo{err: err}
				return
			}
		}
	}()

	return out
}

func (api *PinAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
