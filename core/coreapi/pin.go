package coreapi

import (
	"context"
	"fmt"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	pin "github.com/ipfs/go-ipfs-pinner"
	ipld "github.com/ipfs/go-ipld-format"
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
}

func (p *pinInfo) Path() path.Resolved {
	return p.path
}

func (p *pinInfo) Type() string {
	return p.pinType
}

func (api *PinAPI) pinLsAll(typeStr string, ctx context.Context) ([]coreiface.Pin, error) {
	pinCh, errCh := PinLsAll(ctx, typeStr, api.pinning, api.dag)

	var pins []coreiface.Pin
loop:
	for {
		select {
		case p, ok := <-pinCh:
			if !ok {
				break loop
			}
			pins = append(pins, p)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	err := <-errCh
	if err != nil {
		return nil, err
	}

	return pins, nil
}

// PinLsAll is an internal function for returning a list of pins
func PinLsAll(ctx context.Context, typeStr string, pin pin.Pinner, dag ipld.DAGService) (chan coreiface.Pin, chan error) {
	ch := make(chan coreiface.Pin, 32)
	errCh := make(chan error, 1)

	keys := cid.NewSet()
	AddToResultKeys := func(keyList []cid.Cid, typeStr string) error {
		for _, c := range keyList {
			if keys.Visit(c) {
				select {
				case ch <- &pinInfo{
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

	go func() {
		defer close(ch)
		defer close(errCh)
		if typeStr == "direct" || typeStr == "all" {
			dkeys, err := pin.DirectKeys(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if err := AddToResultKeys(dkeys, "direct"); err != nil {
				errCh <- err
				return
			}
		}
		if typeStr == "recursive" || typeStr == "all" {
			rkeys, err := pin.RecursiveKeys(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if err := AddToResultKeys(rkeys, "recursive"); err != nil {
				errCh <- err
				return
			}
		}
		if typeStr == "indirect" || typeStr == "all" {
			rkeys, err := pin.RecursiveKeys(ctx)
			if err != nil {
				errCh <- err
				return
			}

			// If we're only listing indirect pins, we need to
			// explicitly mark direct/recursive pins so we don't
			// send them.
			if typeStr == "indirect" {
				dkeys, err := pin.DirectKeys(ctx)
				if err != nil {
					errCh <- err
					return
				}

				for _, k := range dkeys {
					keys.Add(k)
				}
				for _, k := range rkeys {
					keys.Add(k)
				}
			}

			indirectKeys := cid.NewSet()
			for _, k := range rkeys {
				err := merkledag.Walk(ctx, merkledag.GetLinksWithDAG(dag), k, func(c cid.Cid) bool {
					r := indirectKeys.Visit(c)
					if r {
						if err := AddToResultKeys([]cid.Cid{c}, "indirect"); err != nil {
							return false
						}
					}
					return r
				}, merkledag.SkipRoot(), merkledag.Concurrent())

				if err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	return ch, errCh
}

func (api *PinAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
