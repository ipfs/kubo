package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	iface "github.com/ipfs/kubo/core/coreiface"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
)

type PinAPI HttpApi

type pinRefKeyObject struct {
	Type string
}

type pinRefKeyList struct {
	Keys map[string]pinRefKeyObject
}

type pin struct {
	path path.ImmutablePath
	typ  string
	name string
	err  error
}

func (p pin) Err() error {
	return p.err
}

func (p pin) Path() path.ImmutablePath {
	return p.path
}

func (p pin) Name() string {
	return p.name
}

func (p pin) Type() string {
	return p.typ
}

func (api *PinAPI) Add(ctx context.Context, p path.Path, opts ...caopts.PinAddOption) error {
	options, err := caopts.PinAddOptions(opts...)
	if err != nil {
		return err
	}

	return api.core().Request("pin/add", p.String()).
		Option("recursive", options.Recursive).Exec(ctx, nil)
}

type pinLsObject struct {
	Cid  string
	Name string
	Type string
}

func (api *PinAPI) Ls(ctx context.Context, pins chan<- iface.Pin, opts ...caopts.PinLsOption) error {
	defer close(pins)

	options, err := caopts.PinLsOptions(opts...)
	if err != nil {
		return err
	}

	res, err := api.core().Request("pin/ls").
		Option("type", options.Type).
		Option("stream", true).
		Send(ctx)
	if err != nil {
		return err
	}
	defer res.Output.Close()

	dec := json.NewDecoder(res.Output)
	var out pinLsObject
	for {
		err := dec.Decode(&out)
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}

		c, err := cid.Parse(out.Cid)
		if err != nil {
			return err
		}

		select {
		case pins <- pin{typ: out.Type, name: out.Name, path: path.FromCid(c)}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// IsPinned returns whether or not the given cid is pinned
// and an explanation of why its pinned.
func (api *PinAPI) IsPinned(ctx context.Context, p path.Path, opts ...caopts.PinIsPinnedOption) (string, bool, error) {
	options, err := caopts.PinIsPinnedOptions(opts...)
	if err != nil {
		return "", false, err
	}
	var out pinRefKeyList
	err = api.core().Request("pin/ls").
		Option("type", options.WithType).
		Option("arg", p.String()).
		Exec(ctx, &out)
	if err != nil {
		// TODO: This error-type discrimination based on sub-string matching is brittle.
		// It is addressed by this open issue: https://github.com/ipfs/go-ipfs/issues/7563
		if strings.Contains(err.Error(), "is not pinned") {
			return "", false, nil
		}
		return "", false, err
	}

	for _, obj := range out.Keys {
		return obj.Type, true, nil
	}
	return "", false, errors.New("http api returned no error and no results")
}

func (api *PinAPI) Rm(ctx context.Context, p path.Path, opts ...caopts.PinRmOption) error {
	options, err := caopts.PinRmOptions(opts...)
	if err != nil {
		return err
	}

	return api.core().Request("pin/rm", p.String()).
		Option("recursive", options.Recursive).
		Exec(ctx, nil)
}

func (api *PinAPI) Update(ctx context.Context, from path.Path, to path.Path, opts ...caopts.PinUpdateOption) error {
	options, err := caopts.PinUpdateOptions(opts...)
	if err != nil {
		return err
	}

	return api.core().Request("pin/update", from.String(), to.String()).
		Option("unpin", options.Unpin).Exec(ctx, nil)
}

type pinVerifyRes struct {
	ok       bool
	badNodes []iface.BadPinNode
	err      error
}

func (r pinVerifyRes) Ok() bool {
	return r.ok
}

func (r pinVerifyRes) BadNodes() []iface.BadPinNode {
	return r.badNodes
}

func (r pinVerifyRes) Err() error {
	return r.err
}

type badNode struct {
	err error
	cid cid.Cid
}

func (n badNode) Path() path.ImmutablePath {
	return path.FromCid(n.cid)
}

func (n badNode) Err() error {
	return n.err
}

func (api *PinAPI) Verify(ctx context.Context) (<-chan iface.PinStatus, error) {
	resp, err := api.core().Request("pin/verify").Option("verbose", true).Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	res := make(chan iface.PinStatus)

	go func() {
		defer resp.Close()
		defer close(res)
		dec := json.NewDecoder(resp.Output)
		for {
			var out struct {
				Cid string
				Err string
				Ok  bool

				BadNodes []struct {
					Cid string
					Err string
				}
			}
			if err := dec.Decode(&out); err != nil {
				if err == io.EOF {
					return
				}
				select {
				case res <- pinVerifyRes{err: err}:
					return
				case <-ctx.Done():
					return
				}
			}

			if out.Err != "" {
				select {
				case res <- pinVerifyRes{err: errors.New(out.Err)}:
					return
				case <-ctx.Done():
					return
				}
			}

			badNodes := make([]iface.BadPinNode, len(out.BadNodes))
			for i, n := range out.BadNodes {
				c, err := cid.Decode(n.Cid)
				if err != nil {
					badNodes[i] = badNode{cid: c, err: err}
					continue
				}

				if n.Err != "" {
					err = errors.New(n.Err)
				}
				badNodes[i] = badNode{cid: c, err: err}
			}

			select {
			case res <- pinVerifyRes{ok: out.Ok, badNodes: badNodes}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return res, nil
}

func (api *PinAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
