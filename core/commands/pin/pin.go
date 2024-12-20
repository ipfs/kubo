package pin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	bserv "github.com/ipfs/boxo/blockservice"
	offline "github.com/ipfs/boxo/exchange/offline"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	verifcid "github.com/ipfs/boxo/verifcid"
	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"

	core "github.com/ipfs/kubo/core"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	e "github.com/ipfs/kubo/core/commands/e"
)

var PinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Pin (and unpin) objects to local storage.",
	},

	Subcommands: map[string]*cmds.Command{
		"add":    addPinCmd,
		"rm":     rmPinCmd,
		"ls":     listPinCmd,
		"verify": verifyPinCmd,
		"update": updatePinCmd,
		"remote": remotePinCmd,
	},
}

type PinOutput struct {
	Pins []string
}

type AddPinOutput struct {
	Pins     []string `json:",omitempty"`
	Progress int      `json:",omitempty"`
}

const (
	pinRecursiveOptionName = "recursive"
	pinProgressOptionName  = "progress"
)

var addPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Pin objects to local storage.",
		ShortDescription: "Stores an IPFS object(s) from a given path locally to disk.",
		LongDescription: `
Create a pin for the given object, protecting resolved CID from being garbage
collected.

An optional name can be provided, and read back via 'ipfs pin ls --names'.

Be mindful of defaults:

Default pin type is 'recursive' (entire DAG).
Pass -r=false to create a direct pin for a single block.
Use 'pin ls -t recursive' to only list roots of recursively pinned DAGs
(significantly faster when many big DAGs are pinned recursively)

Default pin name is empty. Pass '--name' to 'pin add' to set one
and use 'pin ls --names' to see it. Pinning a second time with a different
name will update the name of the pin.

If daemon is running, any missing blocks will be retrieved from the network.
It may take some time. Pass '--progress' to track the progress.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be pinned.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinRecursiveOptionName, "r", "Recursively pin the object linked to by the specified object(s).").WithDefault(true),
		cmds.StringOption(pinNameOptionName, "n", "An optional name for created pin(s)."),
		cmds.BoolOption(pinProgressOptionName, "Show progress"),
	},
	Type: AddPinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// set recursive flag
		recursive, _ := req.Options[pinRecursiveOptionName].(bool)
		name, _ := req.Options[pinNameOptionName].(string)
		showProgress, _ := req.Options[pinProgressOptionName].(bool)

		if err := req.ParseBodyArgs(); err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		if !showProgress {
			added, err := pinAddMany(req.Context, api, enc, req.Arguments, recursive, name)
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
			added, err := pinAddMany(ctx, api, enc, req.Arguments, recursive, name)
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

func pinAddMany(ctx context.Context, api coreiface.CoreAPI, enc cidenc.Encoder, paths []string, recursive bool, name string) ([]string, error) {
	added := make([]string, len(paths))
	for i, b := range paths {
		p, err := cmdutils.PathOrCidPath(b)
		if err != nil {
			return nil, err
		}

		rp, _, err := api.ResolvePath(ctx, p)
		if err != nil {
			return nil, err
		}

		if err := api.Pin().Add(ctx, rp, options.Pin.Recursive(recursive), options.Pin.Name(name)); err != nil {
			return nil, err
		}
		added[i] = enc.Encode(rp.RootCid())
	}

	return added, nil
}

var rmPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove object from pin-list.",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed. (By default, recursively. Use -r=false for direct pins.)
`,
		LongDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed. (By default, recursively. Use -r=false for direct pins.)

A pin may not be removed because the specified object is not pinned or pinned
indirectly. To determine if the object is pinned indirectly, use the command:
ipfs pin ls -t indirect <cid>
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be unpinned.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinRecursiveOptionName, "r", "Recursively unpin the object linked to by the specified object(s).").WithDefault(true),
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
			p, err := cmdutils.PathOrCidPath(b)
			if err != nil {
				return err
			}

			rp, _, err := api.ResolvePath(req.Context, p)
			if err != nil {
				return err
			}

			id := enc.Encode(rp.RootCid())
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
	pinTypeOptionName   = "type"
	pinQuietOptionName  = "quiet"
	pinStreamOptionName = "stream"
	pinNamesOptionName  = "names"
)

var listPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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

By default, pin names are not included (returned as empty).
Pass '--names' flag to return pin names (set with '--name' from 'pin add').

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

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", false, true, "Path to object(s) to be listed."),
	},
	Options: []cmds.Option{
		cmds.StringOption(pinTypeOptionName, "t", "The type of pinned keys to list. Can be \"direct\", \"indirect\", \"recursive\", or \"all\".").WithDefault("all"),
		cmds.BoolOption(pinQuietOptionName, "q", "Output only the CIDs of pins."),
		cmds.StringOption(pinNameOptionName, "n", "Limit returned pins to ones with names that contain the value provided (case-sensitive, partial match). Implies --names=true."),
		cmds.BoolOption(pinStreamOptionName, "s", "Enable streaming of pins as they are discovered."),
		cmds.BoolOption(pinNamesOptionName, "Include pin names in the output (slower, disabled by default)."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		typeStr, _ := req.Options[pinTypeOptionName].(string)
		stream, _ := req.Options[pinStreamOptionName].(bool)
		displayNames, _ := req.Options[pinNamesOptionName].(bool)
		name, _ := req.Options[pinNameOptionName].(string)

		switch typeStr {
		case "all", "direct", "indirect", "recursive":
		default:
			err = fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", typeStr)
			return err
		}

		// For backward compatibility, we accumulate the pins in the same output type as before.
		var emit func(PinLsOutputWrapper) error
		lgcList := map[string]PinLsType{}
		if !stream {
			emit = func(v PinLsOutputWrapper) error {
				lgcList[v.PinLsObject.Cid] = PinLsType{Type: v.PinLsObject.Type, Name: v.PinLsObject.Name}
				return nil
			}
		} else {
			emit = func(v PinLsOutputWrapper) error {
				return res.Emit(v)
			}
		}

		if len(req.Arguments) > 0 {
			err = pinLsKeys(req, typeStr, api, emit)
		} else {
			err = pinLsAll(req, typeStr, displayNames || name != "", name, api, emit)
		}
		if err != nil {
			return err
		}

		if !stream {
			return cmds.EmitOnce(res, PinLsOutputWrapper{
				PinLsList: PinLsList{Keys: lgcList},
			})
		}

		return nil
	},
	Type: PinLsOutputWrapper{},
	Encoders: cmds.EncoderMap{
		cmds.JSON: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out PinLsOutputWrapper) error {
			stream, _ := req.Options[pinStreamOptionName].(bool)

			enc := json.NewEncoder(w)

			if stream {
				return enc.Encode(out.PinLsObject)
			}

			return enc.Encode(out.PinLsList)
		}),
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out PinLsOutputWrapper) error {
			quiet, _ := req.Options[pinQuietOptionName].(bool)
			stream, _ := req.Options[pinStreamOptionName].(bool)

			if stream {
				if quiet {
					fmt.Fprintf(w, "%s\n", out.PinLsObject.Cid)
				} else if out.PinLsObject.Name == "" {
					fmt.Fprintf(w, "%s %s\n", out.PinLsObject.Cid, out.PinLsObject.Type)
				} else {
					fmt.Fprintf(w, "%s %s %s\n", out.PinLsObject.Cid, out.PinLsObject.Type, out.PinLsObject.Name)
				}
				return nil
			}

			for k, v := range out.PinLsList.Keys {
				if quiet {
					fmt.Fprintf(w, "%s\n", k)
				} else if v.Name == "" {
					fmt.Fprintf(w, "%s %s\n", k, v.Type)
				} else {
					fmt.Fprintf(w, "%s %s %s\n", k, v.Type, v.Name)
				}
			}

			return nil
		}),
	},
}

// PinLsOutputWrapper is the output type of the pin ls command.
// Pin ls needs to output two different type depending on if it's streamed or not.
// We use this to bypass the cmds lib refusing to have interface{}
type PinLsOutputWrapper struct {
	PinLsList
	PinLsObject
}

// PinLsList is a set of pins with their type
type PinLsList struct {
	Keys map[string]PinLsType `json:",omitempty"`
}

// PinLsType contains the type of a pin
type PinLsType struct {
	Type string
	Name string
}

// PinLsObject contains the description of a pin
type PinLsObject struct {
	Cid  string `json:",omitempty"`
	Name string `json:",omitempty"`
	Type string `json:",omitempty"`
}

func pinLsKeys(req *cmds.Request, typeStr string, api coreiface.CoreAPI, emit func(value PinLsOutputWrapper) error) error {
	enc, err := cmdenv.GetCidEncoder(req)
	if err != nil {
		return err
	}

	switch typeStr {
	case "all", "direct", "indirect", "recursive":
	default:
		return fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", typeStr)
	}

	opt, err := options.Pin.IsPinned.Type(typeStr)
	if err != nil {
		panic("unhandled pin type")
	}

	for _, p := range req.Arguments {
		p, err := cmdutils.PathOrCidPath(p)
		if err != nil {
			return err
		}

		rp, _, err := api.ResolvePath(req.Context, p)
		if err != nil {
			return err
		}

		pinType, pinned, err := api.Pin().IsPinned(req.Context, rp, opt)
		if err != nil {
			return err
		}

		if !pinned {
			return fmt.Errorf("path '%s' is not pinned", p)
		}

		switch pinType {
		case "direct", "indirect", "recursive", "internal":
		default:
			pinType = "indirect through " + pinType
		}

		err = emit(PinLsOutputWrapper{
			PinLsObject: PinLsObject{
				Type: pinType,
				Cid:  enc.Encode(rp.RootCid()),
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func pinLsAll(req *cmds.Request, typeStr string, detailed bool, name string, api coreiface.CoreAPI, emit func(value PinLsOutputWrapper) error) error {
	enc, err := cmdenv.GetCidEncoder(req)
	if err != nil {
		return err
	}

	switch typeStr {
	case "all", "direct", "indirect", "recursive":
	default:
		err = fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", typeStr)
		return err
	}

	opt, err := options.Pin.Ls.Type(typeStr)
	if err != nil {
		panic("unhandled pin type")
	}

	pins := make(chan coreiface.Pin)
	lsErr := make(chan error, 1)
	lsCtx, cancel := context.WithCancel(req.Context)
	defer cancel()

	go func() {
		lsErr <- api.Pin().Ls(lsCtx, pins, opt, options.Pin.Ls.Detailed(detailed), options.Pin.Ls.Name(name))
	}()

	for p := range pins {
		err = emit(PinLsOutputWrapper{
			PinLsObject: PinLsObject{
				Type: p.Type(),
				Name: p.Name(),
				Cid:  enc.Encode(p.Path().RootCid()),
			},
		})
		if err != nil {
			return err
		}
	}
	return <-lsErr
}

const (
	pinUnpinOptionName = "unpin"
)

var updatePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Update a recursive pin.",
		ShortDescription: `
Efficiently pins a new object based on differences from an existing one and,
by default, removes the old pin.

This command is useful when the new pin contains many similarities or is a
derivative of an existing one, particularly for large objects. This allows a more
efficient DAG-traversal which fully skips already-pinned branches from the old
object. As a requirement, the old object needs to be an existing recursive
pin.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("from-path", true, false, "Path to old object."),
		cmds.StringArg("to-path", true, false, "Path to a new object to be pinned."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinUnpinOptionName, "Remove the old pin.").WithDefault(true),
	},
	Type: PinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		unpin, _ := req.Options[pinUnpinOptionName].(bool)

		fromPath, err := cmdutils.PathOrCidPath(req.Arguments[0])
		if err != nil {
			return err
		}

		toPath, err := cmdutils.PathOrCidPath(req.Arguments[1])
		if err != nil {
			return err
		}

		// Resolve the paths ahead of time so we can return the actual CIDs
		from, _, err := api.ResolvePath(req.Context, fromPath)
		if err != nil {
			return err
		}
		to, _, err := api.ResolvePath(req.Context, toPath)
		if err != nil {
			return err
		}

		err = api.Pin().Update(req.Context, from, to, options.Pin.Unpin(unpin))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &PinOutput{Pins: []string{enc.Encode(from.RootCid()), enc.Encode(to.RootCid())}})
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
	Helptext: cmds.HelpText{
		Tagline: "Verify that recursive pins are complete.",
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinVerboseOptionName, "Also write the hashes of non-broken pins."),
		cmds.BoolOption(pinQuietOptionName, "q", "Write just hashes of broken pins."),
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
		out, err := pinVerify(req.Context, n, opts, enc)
		if err != nil {
			return err
		}
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

// PinVerifyRes is the result returned for each pin checked in "pin verify"
type PinVerifyRes struct {
	Cid string `json:",omitempty"`
	Err string `json:",omitempty"`
	PinStatus
}

// PinStatus is part of PinVerifyRes, do not use directly
type PinStatus struct {
	Ok       bool      `json:",omitempty"`
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

// FIXME: this implementation is duplicated sith core/coreapi.PinAPI.Verify, remove this one and exclusively rely on CoreAPI.
func pinVerify(ctx context.Context, n *core.IpfsNode, opts pinVerifyOpts, enc cidenc.Encoder) (<-chan any, error) {
	visited := make(map[cid.Cid]PinStatus)

	bs := n.Blocks.Blockstore()
	DAG := dag.NewDAGService(bserv.New(bs, offline.Exchange(bs)))
	getLinks := dag.GetLinksWithDAG(DAG)

	var checkPin func(root cid.Cid) PinStatus
	checkPin = func(root cid.Cid) PinStatus {
		key := root
		if status, ok := visited[key]; ok {
			return status
		}

		if err := verifcid.ValidateCid(verifcid.DefaultAllowlist, root); err != nil {
			status := PinStatus{Ok: false}
			if opts.explain {
				status.BadNodes = []BadNode{{Cid: enc.Encode(key), Err: err.Error()}}
			}
			visited[key] = status
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := PinStatus{Ok: false}
			if opts.explain {
				status.BadNodes = []BadNode{{Cid: enc.Encode(key), Err: err.Error()}}
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

	out := make(chan any)
	go func() {
		defer close(out)
		for p := range n.Pinning.RecursiveKeys(ctx, false) {
			if p.Err != nil {
				out <- PinVerifyRes{Err: p.Err.Error()}
				return
			}
			pinStatus := checkPin(p.Pin.Key)
			if !pinStatus.Ok || opts.includeOk {
				select {
				case out <- PinVerifyRes{Cid: enc.Encode(p.Pin.Key), PinStatus: pinStatus}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

// Format formats PinVerifyRes
func (r PinVerifyRes) Format(out io.Writer) {
	if r.Err != "" {
		fmt.Fprintf(out, "error: %s\n", r.Err)
		return
	}

	if r.Ok {
		fmt.Fprintf(out, "%s ok\n", r.Cid)
		return
	}

	fmt.Fprintf(out, "%s broken\n", r.Cid)
	for _, e := range r.BadNodes {
		fmt.Fprintf(out, "  %s: %s\n", e.Cid, e.Err)
	}
}
