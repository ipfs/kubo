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

func (api *PinAPI) Add(ctx context.Context, pinpath string, p path.Path, opts ...caopts.PinAddOption) error {
	settings, err := caopts.PinAddOptions(opts...)
	if err != nil {
		return nil
	}

	dagNode, err := api.core().ResolveNode(ctx, p)

	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	err = api.pinning.Pin(ctx, pinpath, dagNode, settings.Recursive)

	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	if err := api.provider.Provide(dagNode.Cid()); err != nil {
		return err
	}

	return nil
}

func (api *PinAPI) Ls(ctx context.Context, prefix string, opts ...caopts.PinLsOption) (<-chan coreiface.Pin, error) {
	settings, err := caopts.PinLsOptions(opts...)
	if err != nil {
		return nil, err
	}

	switch settings.Type {
	case "all", "direct", "indirect", "recursive":
	default:
		return nil, fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", settings.Type)
	}

	return api.pinLsAll(settings.Type, prefix, ctx, settings.Recursive), nil
}

// Rm pin rm api
func (api *PinAPI) Rm(ctx context.Context, p string, opts ...caopts.PinRmOption) error {

	settings, err := caopts.PinRmOptions(opts...)
	if err != nil {
		return err
	}

	return api.pinning.Unpin(p, settings.Recursive)
}

func (api *PinAPI) Update(ctx context.Context, from string, to path.Path, opts ...caopts.PinUpdateOption) error {
	tp, err := api.core().ResolvePath(ctx, to)
	if err != nil {
		return err
	}

	return api.pinning.Update(ctx, from, tp.Cid())
}

type pinStatus struct {
	pinPath  string
	ok       bool
	badNodes []coreiface.BadPinNode
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

	pinneds, err := api.pinning.CheckIfPinned(ctx, resolved.Cid())
	if err != nil {
		return "", false, err
	}

	for _, pinned := range pinneds {
		if settings.WithType == "all" ||
			settings.WithType == "direct" && pinned.Mode == pin.Direct ||
			settings.WithType == "indirect" && pinned.Mode == pin.Indirect ||
			settings.WithType == "recursive" && pinned.Mode == pin.Recursive {
			return pinned.String(), true, nil
		}
	}
	return "not pinned", false, nil
}

// BadNode is used in PinVerifyRes
type badNode struct {
	path path.Resolved
	err  error
}

func (s *pinStatus) Ok() bool {
	return s.ok
}

func (s *pinStatus) PinPath() string {
	return s.pinPath
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
	recPins, err := api.pinning.PrefixedPins("", true)

	if err != nil {
		return nil, err
	}

	var checkPin func(root cid.Cid, pinPath string) *pinStatus
	checkPin = func(root cid.Cid, pinPath string) *pinStatus {
		status, ok := visited[root]
		if ok {
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := &pinStatus{ok: false, pinPath: pinPath}
			status.badNodes = []coreiface.BadPinNode{&badNode{path: path.IpldPath(root), err: err}}
			visited[root] = status
			return status
		}

		status = &pinStatus{ok: true}
		for _, lnk := range links {
			res := checkPin(lnk.Cid, pinPath)
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
		for path, c := range recPins {
			out <- checkPin(c, path)
		}
	}()

	return out, nil
}

type pinInfo struct {
	pinType string
	pinPath string
	path    path.Resolved
	err     error
}

func (p *pinInfo) PinPath() string {
	return p.pinPath
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
func (api *PinAPI) pinLsAll(typeStr string, prefix string, ctx context.Context, recursive bool) <-chan coreiface.Pin {
	out := make(chan coreiface.Pin)

	var recursiveMap map[string]cid.Cid
	var directMap map[string]cid.Cid

	if api.nd == nil {
		out <- &pinInfo{err: fmt.Errorf("cannot apply options to api without node")}
		return out
	}

	n := api.nd

	if recursive {
		var err error
		recursiveMap, err = n.Pinning.PrefixedPins(prefix, true)
		if err != nil {
			out <- &pinInfo{err: err}
			return out
		}
		directMap, err = n.Pinning.PrefixedPins(prefix, false)
		if err != nil {
			out <- &pinInfo{err: err}
			return out
		}
	} else {
		recursiveMap = make(map[string]cid.Cid)
		c, err := n.Pinning.GetPin(prefix, true)
		if err != pin.ErrNotPinned && err != nil {
			out <- &pinInfo{err: err}
			return out
		} else if err != pin.ErrNotPinned {
			recursiveMap[prefix] = *c
		}
		directMap = make(map[string]cid.Cid)
		c, err = n.Pinning.GetPin(prefix, false)
		if err != pin.ErrNotPinned && err != nil {
			out <- &pinInfo{err: err}
			return out
		} else if err != pin.ErrNotPinned {
			directMap[prefix] = *c
		}
	}

	go func() {
		defer close(out)

		if typeStr == "recursive" || typeStr == "all" {
			for pinPath, c := range recursiveMap {
				out <- &pinInfo{
					pinType: "recursive",
					path:    path.IpldPath(c),
					pinPath: pinPath,
				}
			}
		}
		if typeStr == "direct" || typeStr == "all" {
			for pinPath, c := range directMap {
				out <- &pinInfo{
					pinType: "direct",
					path:    path.IpldPath(c),
					pinPath: pinPath,
				}
			}
		}

		if typeStr == "indirect" || typeStr == "all" {
			for pinPath, c := range recursiveMap {
				set := cid.NewSet()
				err := merkledag.Walk(ctx, merkledag.GetLinksWithDAG(api.dag), c, set.Visit,
					merkledag.SkipRoot(), merkledag.Concurrent())
				if err != nil {
					out <- &pinInfo{err: err}
					return
				}
				for _, k := range set.Keys() {
					out <- &pinInfo{
						pinType: "indirect",
						pinPath: pinPath,
						path:    path.IpldPath(k),
					}
				}
			}
		}

	}()

	return out
}

func (api *PinAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
