package coreapi

import (
	"context"
	"fmt"
	"io"

	"github.com/Jorropo/channel"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	pin "github.com/ipfs/go-ipfs-pinner"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-verifcid"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs/kubo/tracing"
)

type PinAPI CoreAPI

func (api *PinAPI) Add(ctx context.Context, p path.Path, opts ...caopts.PinAddOption) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinAPI", "Add", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	dagNode, err := api.core().ResolveNode(ctx, p)
	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	settings, err := caopts.PinAddOptions(opts...)
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.Bool("recursive", settings.Recursive))

	defer api.blockstore.PinLock(ctx).Unlock(ctx)

	err = api.pinning.Pin(ctx, dagNode, settings.Recursive)
	if err != nil {
		return fmt.Errorf("pin: %s", err)
	}

	if err := api.provider.Provide(dagNode.Cid()); err != nil {
		return err
	}

	return api.pinning.Flush(ctx)
}

func (api *PinAPI) Ls(ctx context.Context, opts ...caopts.PinLsOption) channel.ReadOnly[coreiface.Pin] {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinAPI", "Ls")
	defer span.End()

	settings, err := caopts.PinLsOptions(opts...)
	if err != nil {
		return channel.NewWithError[coreiface.Pin](err).ReadOnly()
	}

	span.SetAttributes(attribute.String("type", settings.Type))

	switch settings.Type {
	case "all", "direct", "indirect", "recursive":
	default:
		err := fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", settings.Type)
		return channel.NewWithError[coreiface.Pin](err).ReadOnly()
	}

	return api.pinLsAll(ctx, settings.Type)
}

func (api *PinAPI) IsPinned(ctx context.Context, p path.Path, opts ...caopts.PinIsPinnedOption) (string, bool, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinAPI", "IsPinned", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	resolved, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return "", false, fmt.Errorf("error resolving path: %s", err)
	}

	settings, err := caopts.PinIsPinnedOptions(opts...)
	if err != nil {
		return "", false, err
	}

	span.SetAttributes(attribute.String("withtype", settings.WithType))

	mode, ok := pin.StringToMode(settings.WithType)
	if !ok {
		return "", false, fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", settings.WithType)
	}

	return api.pinning.IsPinnedWithType(ctx, resolved.Cid(), mode)
}

// Rm pin rm api
func (api *PinAPI) Rm(ctx context.Context, p path.Path, opts ...caopts.PinRmOption) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinAPI", "Rm", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return err
	}

	settings, err := caopts.PinRmOptions(opts...)
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.Bool("recursive", settings.Recursive))

	// Note: after unpin the pin sets are flushed to the blockstore, so we need
	// to take a lock to prevent a concurrent garbage collection
	defer api.blockstore.PinLock(ctx).Unlock(ctx)

	if err = api.pinning.Unpin(ctx, rp.Cid(), settings.Recursive); err != nil {
		return err
	}

	return api.pinning.Flush(ctx)
}

func (api *PinAPI) Update(ctx context.Context, from path.Path, to path.Path, opts ...caopts.PinUpdateOption) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinAPI", "Update", trace.WithAttributes(
		attribute.String("from", from.String()),
		attribute.String("to", to.String()),
	))
	defer span.End()

	settings, err := caopts.PinUpdateOptions(opts...)
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.Bool("unpin", settings.Unpin))

	fp, err := api.core().ResolvePath(ctx, from)
	if err != nil {
		return err
	}

	tp, err := api.core().ResolvePath(ctx, to)
	if err != nil {
		return err
	}

	defer api.blockstore.PinLock(ctx).Unlock(ctx)

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

func (s *pinStatus) Cid() cid.Cid {
	return s.cid
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

func (api *PinAPI) Verify(ctx context.Context, opts ...caopts.PinVerifyOption) channel.ReadOnly[coreiface.PinStatus] {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinAPI", "Verify")
	defer span.End()

	settings, err := caopts.PinVerifyOptions(opts...)
	if err != nil {
		return channel.NewWithError[coreiface.PinStatus](err).ReadOnly()
	}

	visited := make(map[cid.Cid]*pinStatus)
	bs := api.blockstore
	DAG := merkledag.NewDAGService(bserv.New(bs, offline.Exchange(bs)))
	getLinks := merkledag.GetLinksWithDAG(DAG)

	var checkPin func(root cid.Cid) *pinStatus
	checkPin = func(root cid.Cid) *pinStatus {
		ctx, span := tracing.Span(ctx, "CoreAPI.PinAPI", "Verify.CheckPin", trace.WithAttributes(attribute.String("cid", root.String())))
		defer span.End()

		if status, ok := visited[root]; ok {
			return status
		}

		if err := verifcid.ValidateCid(root); err != nil {
			status := &pinStatus{ok: false, cid: root}
			if settings.Explain {
				status.badNodes = []coreiface.BadPinNode{&badNode{path: path.IpldPath(root), err: err}}
			}
			visited[root] = status
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := &pinStatus{ok: false, cid: root}
			if settings.Explain {
				status.badNodes = []coreiface.BadPinNode{&badNode{path: path.IpldPath(root), err: err}}
			}
			visited[root] = status
			return status
		}

		status := &pinStatus{ok: true, cid: root}
		for _, lnk := range links {
			res := checkPin(lnk.Cid)
			if !res.ok || settings.IncludeOk {
				status.ok = res.ok
				if settings.Explain {
					status.badNodes = append(status.badNodes, res.badNodes...)
				}
			}
		}

		visited[root] = status
		return status
	}

	recPins := api.pinning.RecursiveKeys(ctx)

	out := channel.New[coreiface.PinStatus]()
	go func() {
		defer out.Close()
		for {
			c, err := recPins.ReadContext(ctx)
			if err == io.EOF {
				break
			}
			if err != nil {
				out.SetError(err)
				break
			}
			err = out.WriteContext(ctx, checkPin(c))
			if err != nil {
				break
			}
		}
	}()

	return out.ReadOnly()
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
//
// The caller must keep reading results until the channel is closed to prevent
// leaking the goroutine that is fetching pins.
func (api *PinAPI) pinLsAll(ctx context.Context, typeStr string) channel.ReadOnly[coreiface.Pin] {
	out := channel.NewWithSize[coreiface.Pin](1)

	keySet := cid.NewSet()

	AddToResultKeys := func(c cid.Cid, typeStr string) error {
		if keySet.Visit(c) {
			err := out.WriteContext(ctx, &pinInfo{
				pinType: typeStr,
				path:    path.IpldPath(c),
			})
			if err != nil {
				return err
			}
		}
		return nil
	}

	go func() {
		defer out.Close()

		var rkeys []cid.Cid
		var err error
		if typeStr == "recursive" || typeStr == "all" {
			err = api.pinning.RecursiveKeys(ctx).RangeContext(ctx, func(c cid.Cid) error {
				if typeStr == "all" {
					rkeys = append(rkeys, c)
				}
				return AddToResultKeys(c, "recursive")
			})
			if err != nil {
				out.SetError(err)
				return
			}
		}
		if typeStr == "direct" || typeStr == "all" {
			err = api.pinning.DirectKeys(ctx).RangeContext(ctx, func(c cid.Cid) error {
				return AddToResultKeys(c, "direct")
			})
			if err != nil {
				out.SetError(err)
				return
			}
		}
		if typeStr == "all" {
			set := cid.NewSet()
			for _, k := range rkeys {
				err = merkledag.Walk(
					ctx, merkledag.GetLinksWithDAG(api.dag), k,
					set.Visit,
					merkledag.SkipRoot(), merkledag.Concurrent(),
				)
				if err != nil {
					out.SetError(err)
					return
				}
			}
			err = set.ForEach(func(c cid.Cid) error {
				return AddToResultKeys(c, "indirect")
			})
			if err != nil {
				out.SetError(err)
				return
			}
		}
		if typeStr == "indirect" {
			// We need to first visit the direct pins that have priority
			// without emitting them

			err = api.pinning.DirectKeys(ctx).RangeContext(ctx, func(c cid.Cid) error {
				keySet.Add(c)
				return nil
			})
			if err != nil {
				out.SetError(err)
				return
			}

			err = api.pinning.RecursiveKeys(ctx).RangeContext(ctx, func(c cid.Cid) error {
				if keySet.Visit(c) {
					rkeys = append(rkeys, c)
				}
				return nil
			})
			if err != nil {
				out.SetError(err)
				return
			}

			set := cid.NewSet()
			for _, k := range rkeys {
				err = merkledag.Walk(
					ctx, merkledag.GetLinksWithDAG(api.dag), k,
					set.Visit,
					merkledag.SkipRoot(), merkledag.Concurrent(),
				)
				if err != nil {
					out.SetError(err)
					return
				}
			}
			err = set.ForEach(func(c cid.Cid) error {
				return AddToResultKeys(c, "indirect")
			})
			if err != nil {
				out.SetError(err)
				return
			}
		}
	}()

	return out.ReadOnly()
}

func (api *PinAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
