package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	pin "github.com/ipfs/go-ipfs/pin"

	bserv "github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	dag "github.com/ipfs/go-merkledag"
	verifcid "github.com/ipfs/go-verifcid"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
)

var PinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Pin (and unpin) objects to local storage.",
	},

	Subcommands: map[string]*cmds.Command{
		"add":    addPinCmd,
		"rm":     rmPinCmd,
		"ls":     listPinCmd,
		"verify": verifyPinCmd,
		"update": updatePinCmd,
	},
}

type PinOutput struct {
	Pins []string
}

type AddPinOutput struct {
	Pins     []string
	Progress int `json:",omitempty"`
}

const (
	pinRecursiveOptionName = "recursive"
	pinProgressOptionName  = "progress"
)

var addPinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Pin objects to local storage.",
		ShortDescription: "Stores an IPFS object(s) from a given path locally to disk.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "Path to object(s) to be pinned.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pinRecursiveOptionName, "r", "Recursively pin the object linked to by the specified object(s).").WithDefault(true),
		cmdkit.BoolOption(pinProgressOptionName, "Show progress"),
	},
	Type: AddPinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// set recursive flag
		recursive, _ := req.Options[pinRecursiveOptionName].(bool)
		showProgress, _ := req.Options[pinProgressOptionName].(bool)

		if err := req.ParseBodyArgs(); err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		if !showProgress {
			added, err := pinAddMany(req.Context, api, enc, req.Arguments, recursive)
			if err != nil {
				return err
			}

			return cmds.EmitOnce(res, &AddPinOutput{Pins: added})
		}

		v := new(dag.ProgressTracker)
		ctx := v.DeriveContext(req.Context)

		type pinResult struct {
			pins []string
			err  error
		}

		ch := make(chan pinResult, 1)
		go func() {
			added, err := pinAddMany(ctx, api, enc, req.Arguments, recursive)
			ch <- pinResult{pins: added, err: err}
		}()

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case val := <-ch:
				if val.err != nil {
					return val.err
				}

				if pv := v.Value(); pv != 0 {
					if err := res.Emit(&AddPinOutput{Progress: v.Value()}); err != nil {
						return err
					}
				}
				return res.Emit(&AddPinOutput{Pins: val.pins})
			case <-ticker.C:
				if err := res.Emit(&AddPinOutput{Progress: v.Value()}); err != nil {
					return err
				}
			case <-ctx.Done():
				log.Error(ctx.Err())
				return ctx.Err()
			}
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *AddPinOutput) error {
			rec, found := req.Options["recursive"].(bool)
			var pintype string
			if rec || !found {
				pintype = "recursively"
			} else {
				pintype = "directly"
			}

			for _, k := range out.Pins {
				fmt.Fprintf(w, "pinned %s %s\n", k, pintype)
			}

			return nil
		}),
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}

				out, ok := v.(*AddPinOutput)
				if !ok {
					return e.TypeErr(out, v)
				}
				if out.Pins == nil {
					// this can only happen if the progress option is set
					fmt.Fprintf(os.Stderr, "Fetched/Processed %d nodes\r", out.Progress)
				} else {
					err = re.Emit(out)
					if err != nil {
						return err
					}
				}
			}
		},
	},
}

func pinAddMany(ctx context.Context, api coreiface.CoreAPI, enc cidenc.Encoder, paths []string, recursive bool) ([]string, error) {
	added := make([]string, len(paths))
	for i, b := range paths {
		p, err := coreiface.ParsePath(b)
		if err != nil {
			return nil, err
		}

		rp, err := api.ResolvePath(ctx, p)
		if err != nil {
			return nil, err
		}

		if err := api.Pin().Add(ctx, rp, options.Pin.Recursive(recursive)); err != nil {
			return nil, err
		}
		added[i] = enc.Encode(rp.Cid())
	}

	return added, nil
}

var rmPinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove pinned objects from local storage.",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed. (By default, recursively. Use -r=false for direct pins.)
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "Path to object(s) to be unpinned.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pinRecursiveOptionName, "r", "Recursively unpin the object linked to by the specified object(s).").WithDefault(true),
	},
	Type: PinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// set recursive flag
		recursive, _ := req.Options[pinRecursiveOptionName].(bool)

		if err := req.ParseBodyArgs(); err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		pins := make([]string, 0, len(req.Arguments))
		for _, b := range req.Arguments {
			p, err := coreiface.ParsePath(b)
			if err != nil {
				return err
			}

			rp, err := api.ResolvePath(req.Context, p)
			if err != nil {
				return err
			}

			id := enc.Encode(rp.Cid())
			pins = append(pins, id)
			if err := api.Pin().Rm(req.Context, rp, options.Pin.RmRecursive(recursive)); err != nil {
				return err
			}
		}

		return cmds.EmitOnce(res, &PinOutput{pins})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *PinOutput) error {
			for _, k := range out.Pins {
				fmt.Fprintf(w, "unpinned %s\n", k)
			}

			return nil
		}),
	},
}

const (
	pinTypeOptionName  = "type"
	pinQuietOptionName = "quiet"
)

var listPinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List objects pinned to local storage.",
		ShortDescription: `
Returns a list of objects that are pinned locally.
By default, all pinned objects are returned, but the '--type' flag or
arguments can restrict that to a specific pin type or to some specific objects
respectively.
`,
		LongDescription: `
Returns a list of objects that are pinned locally.
By default, all pinned objects are returned, but the '--type' flag or
arguments can restrict that to a specific pin type or to some specific objects
respectively.

Use --type=<type> to specify the type of pinned keys to list.
Valid values are:
    * "direct": pin that specific object.
    * "recursive": pin that specific object, and indirectly pin all its
    	descendants
    * "indirect": pinned indirectly by an ancestor (like a refcount)
    * "all"

With arguments, the command fails if any of the arguments is not a pinned
object. And if --type=<type> is additionally used, the command will also fail
if any of the arguments is not of the specified type.

Example:
	$ echo "hello" | ipfs add -q
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	$ ipfs pin ls
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN recursive
	# now remove the pin, and repin it directly
	$ ipfs pin rm QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	unpinned QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	$ ipfs pin add -r=false QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	pinned QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN directly
	$ ipfs pin ls --type=direct
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN direct
	$ ipfs pin ls QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN direct
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", false, true, "Path to object(s) to be listed."),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(pinTypeOptionName, "t", "The type of pinned keys to list. Can be \"direct\", \"indirect\", \"recursive\", or \"all\".").WithDefault("all"),
		cmdkit.BoolOption(pinQuietOptionName, "q", "Write just hashes of objects."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		typeStr, _ := req.Options[pinTypeOptionName].(string)
		if err != nil {
			return err
		}

		switch typeStr {
		case "all", "direct", "indirect", "recursive":
		default:
			err = fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", typeStr)
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		var keys map[cid.Cid]RefKeyObject
		if len(req.Arguments) > 0 {
			keys, err = pinLsKeys(req.Context, req.Arguments, typeStr, n, api)
		} else {
			keys, err = pinLsAll(req.Context, typeStr, n)
		}
		if err != nil {
			return err
		}

		refKeys := make(map[string]RefKeyObject, len(keys))
		for k, v := range keys {
			refKeys[enc.Encode(k)] = v
		}

		return cmds.EmitOnce(res, &RefKeyList{Keys: refKeys})
	},
	Type: RefKeyList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *RefKeyList) error {
			quiet, _ := req.Options[pinQuietOptionName].(bool)

			for k, v := range out.Keys {
				if quiet {
					fmt.Fprintf(w, "%s\n", k)
				} else {
					fmt.Fprintf(w, "%s %s\n", k, v.Type)
				}
			}

			return nil
		}),
	},
}

const (
	pinUnpinOptionName = "unpin"
)

var updatePinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Update a recursive pin",
		ShortDescription: `
Updates one pin to another, making sure that all objects in the new pin are
local.  Then removes the old pin. This is an optimized version of adding the
new pin and removing the old one.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("from-path", true, false, "Path to old object."),
		cmdkit.StringArg("to-path", true, false, "Path to new object to be pinned."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pinUnpinOptionName, "Remove the old pin.").WithDefault(true),
	},
	Type: PinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		unpin, _ := req.Options[pinUnpinOptionName].(bool)

		from, err := coreiface.ParsePath(req.Arguments[0])
		if err != nil {
			return err
		}

		to, err := coreiface.ParsePath(req.Arguments[1])
		if err != nil {
			return err
		}

		err = api.Pin().Update(req.Context, from, to, options.Pin.Unpin(unpin))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &PinOutput{Pins: []string{from.String(), to.String()}})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *PinOutput) error {
			fmt.Fprintf(w, "updated %s to %s\n", out.Pins[0], out.Pins[1])
			return nil
		}),
	},
}

const (
	pinVerboseOptionName = "verbose"
)

var verifyPinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Verify that recursive pins are complete.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pinVerboseOptionName, "Also write the hashes of non-broken pins."),
		cmdkit.BoolOption(pinQuietOptionName, "q", "Write just hashes of broken pins."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		verbose, _ := req.Options[pinVerboseOptionName].(bool)
		quiet, _ := req.Options[pinQuietOptionName].(bool)

		if verbose && quiet {
			return fmt.Errorf("the --verbose and --quiet options can not be used at the same time")
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		opts := pinVerifyOpts{
			explain:   !quiet,
			includeOk: verbose,
		}
		out := pinVerify(req.Context, n, opts, enc)

		return res.Emit(out)
	},
	Type: PinVerifyRes{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *PinVerifyRes) error {
			quiet, _ := req.Options[pinQuietOptionName].(bool)

			if quiet && !out.Ok {
				fmt.Fprintf(w, "%s\n", out.Cid)
			} else if !quiet {
				out.Format(w)
			}

			return nil
		}),
	},
}

type RefKeyObject struct {
	Type string
}

type RefKeyList struct {
	Keys map[string]RefKeyObject
}

func pinLsKeys(ctx context.Context, args []string, typeStr string, n *core.IpfsNode, api coreiface.CoreAPI) (map[cid.Cid]RefKeyObject, error) {

	mode, ok := pin.StringToMode(typeStr)
	if !ok {
		return nil, fmt.Errorf("invalid pin mode '%s'", typeStr)
	}

	keys := make(map[cid.Cid]RefKeyObject)

	for _, p := range args {
		pth, err := coreiface.ParsePath(p)
		if err != nil {
			return nil, err
		}

		c, err := api.ResolvePath(ctx, pth)
		if err != nil {
			return nil, err
		}

		pinType, pinned, err := n.Pinning.IsPinnedWithType(c.Cid(), mode)
		if err != nil {
			return nil, err
		}

		if !pinned {
			return nil, fmt.Errorf("path '%s' is not pinned", p)
		}

		switch pinType {
		case "direct", "indirect", "recursive", "internal":
		default:
			pinType = "indirect through " + pinType
		}
		keys[c.Cid()] = RefKeyObject{
			Type: pinType,
		}
	}

	return keys, nil
}

func pinLsAll(ctx context.Context, typeStr string, n *core.IpfsNode) (map[cid.Cid]RefKeyObject, error) {

	keys := make(map[cid.Cid]RefKeyObject)

	AddToResultKeys := func(keyList []cid.Cid, typeStr string) {
		for _, c := range keyList {
			keys[c] = RefKeyObject{
				Type: typeStr,
			}
		}
	}

	if typeStr == "direct" || typeStr == "all" {
		AddToResultKeys(n.Pinning.DirectKeys(), "direct")
	}
	if typeStr == "indirect" || typeStr == "all" {
		set := cid.NewSet()
		for _, k := range n.Pinning.RecursiveKeys() {
			err := dag.EnumerateChildren(ctx, dag.GetLinksWithDAG(n.DAG), k, set.Visit)
			if err != nil {
				return nil, err
			}
		}
		AddToResultKeys(set.Keys(), "indirect")
	}
	if typeStr == "recursive" || typeStr == "all" {
		AddToResultKeys(n.Pinning.RecursiveKeys(), "recursive")
	}

	return keys, nil
}

// PinVerifyRes is the result returned for each pin checked in "pin verify"
type PinVerifyRes struct {
	Cid string
	PinStatus
}

// PinStatus is part of PinVerifyRes, do not use directly
type PinStatus struct {
	Ok       bool
	BadNodes []BadNode `json:",omitempty"`
}

// BadNode is used in PinVerifyRes
type BadNode struct {
	Cid string
	Err string
}

type pinVerifyOpts struct {
	explain   bool
	includeOk bool
}

func pinVerify(ctx context.Context, n *core.IpfsNode, opts pinVerifyOpts, enc cidenc.Encoder) <-chan interface{} {
	visited := make(map[cid.Cid]PinStatus)

	bs := n.Blocks.Blockstore()
	DAG := dag.NewDAGService(bserv.New(bs, offline.Exchange(bs)))
	getLinks := dag.GetLinksWithDAG(DAG)
	recPins := n.Pinning.RecursiveKeys()

	var checkPin func(root cid.Cid) PinStatus
	checkPin = func(root cid.Cid) PinStatus {
		key := root
		if status, ok := visited[key]; ok {
			return status
		}

		if err := verifcid.ValidateCid(root); err != nil {
			status := PinStatus{Ok: false}
			if opts.explain {
				status.BadNodes = []BadNode{BadNode{Cid: enc.Encode(key), Err: err.Error()}}
			}
			visited[key] = status
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := PinStatus{Ok: false}
			if opts.explain {
				status.BadNodes = []BadNode{BadNode{Cid: enc.Encode(key), Err: err.Error()}}
			}
			visited[key] = status
			return status
		}

		status := PinStatus{Ok: true}
		for _, lnk := range links {
			res := checkPin(lnk.Cid)
			if !res.Ok {
				status.Ok = false
				status.BadNodes = append(status.BadNodes, res.BadNodes...)
			}
		}

		visited[key] = status
		return status
	}

	out := make(chan interface{})
	go func() {
		defer close(out)
		for _, cid := range recPins {
			pinStatus := checkPin(cid)
			if !pinStatus.Ok || opts.includeOk {
				select {
				case out <- &PinVerifyRes{enc.Encode(cid), pinStatus}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

// Format formats PinVerifyRes
func (r PinVerifyRes) Format(out io.Writer) {
	if r.Ok {
		fmt.Fprintf(out, "%s ok\n", r.Cid)
	} else {
		fmt.Fprintf(out, "%s broken\n", r.Cid)
		for _, e := range r.BadNodes {
			fmt.Fprintf(out, "  %s: %s\n", e.Cid, e.Err)
		}
	}
}
