package dagcmd

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/coredag"
	iface "github.com/ipfs/interface-go-ipfs-core"

	blk "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	mdag "github.com/ipfs/go-merkledag"
	ipfspath "github.com/ipfs/go-path"
	"github.com/ipfs/interface-go-ipfs-core/options"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	mh "github.com/multiformats/go-multihash"

	gocar "github.com/ipld/go-car"
	gipfree "github.com/ipld/go-ipld-prime/impl/free"
	gipselector "github.com/ipld/go-ipld-prime/traversal/selector"
	gipselectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"
)

const (
	progressOptionName = "progress"
	silentOptionName   = "silent"
	pinRootsOptionName = "pin-roots"
)

var DagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with ipld dag objects.",
		ShortDescription: `
'ipfs dag' is used for creating and manipulating dag objects/hierarchies.

This subcommand is currently an experimental feature, but it is intended
to deprecate and replace the existing 'ipfs object' command moving forward.
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"put":     DagPutCmd,
		"get":     DagGetCmd,
		"resolve": DagResolveCmd,
		"import":  DagImportCmd,
		"export":  DagExportCmd,
	},
}

// OutputObject is the output type of 'dag put' command
type OutputObject struct {
	Cid cid.Cid
}

// ResolveOutput is the output type of 'dag resolve' command
type ResolveOutput struct {
	Cid     cid.Cid
	RemPath string
}

// CarImportOutput is the output type of the 'dag import' commands
type CarImportOutput struct {
	Root         RootMeta
	ObjectCounts ObjectCounts `json:"-"`
}
type RootMeta struct {
	Cid          *cid.Cid
	PresentInCar bool
	PinErrorMsg  string
}
type ObjectCounts struct {
	Blocks        int64
	RootsDeclared int64
	RootsSeen     int64
}

var DagPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a dag node to ipfs.",
		ShortDescription: `
'ipfs dag put' accepts input from a file or stdin and parses it
into an object of the specified format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("object data", true, true, "The object to put").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "f", "Format that the object will be added as.").WithDefault("cbor"),
		cmds.StringOption("input-enc", "Format that the input object will be.").WithDefault("json"),
		cmds.BoolOption("pin", "Pin this object when adding."),
		cmds.StringOption("hash", "Hash function to use").WithDefault(""),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		ienc, _ := req.Options["input-enc"].(string)
		format, _ := req.Options["format"].(string)
		hash, _ := req.Options["hash"].(string)
		dopin, _ := req.Options["pin"].(bool)

		// mhType tells inputParser which hash should be used. MaxUint64 means 'use
		// default hash' (sha256 for cbor, sha1 for git..)
		mhType := uint64(math.MaxUint64)

		if hash != "" {
			var ok bool
			mhType, ok = mh.Names[hash]
			if !ok {
				return fmt.Errorf("%s in not a valid multihash name", hash)
			}
		}

		var adder ipld.NodeAdder = api.Dag()
		if dopin {
			adder = api.Dag().Pinning()
		}
		b := ipld.NewBatch(req.Context, adder)

		it := req.Files.Entries()
		for it.Next() {
			file := files.FileFromEntry(it)
			if file == nil {
				return fmt.Errorf("expected a regular file")
			}
			nds, err := coredag.ParseInputs(ienc, format, file, mhType, -1)
			if err != nil {
				return err
			}
			if len(nds) == 0 {
				return fmt.Errorf("no node returned from ParseInputs")
			}

			for _, nd := range nds {
				err := b.Add(req.Context, nd)
				if err != nil {
					return err
				}
			}

			cid := nds[0].Cid()
			if err := res.Emit(&OutputObject{Cid: cid}); err != nil {
				return err
			}
		}
		if it.Err() != nil {
			return it.Err()
		}

		if err := b.Commit(); err != nil {
			return err
		}

		return nil
	},
	Type: OutputObject{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *OutputObject) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, enc.Encode(out.Cid))
			return nil
		}),
	},
}

var DagGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a dag node from ipfs.",
		ShortDescription: `
'ipfs dag get' fetches a dag node from ipfs and prints it out in the specified
format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The object to get").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		rp, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		obj, err := api.Dag().Get(req.Context, rp.Cid())
		if err != nil {
			return err
		}

		var out interface{} = obj
		if len(rp.Remainder()) > 0 {
			rem := strings.Split(rp.Remainder(), "/")
			final, _, err := obj.Resolve(rem)
			if err != nil {
				return err
			}
			out = final
		}
		return cmds.EmitOnce(res, &out)
	},
}

// DagResolveCmd returns address of highest block within a path and a path remainder
var DagResolveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Resolve ipld block",
		ShortDescription: `
'ipfs dag resolve' fetches a dag node from ipfs, prints it's address and remaining path.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The path to resolve").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		rp, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ResolveOutput{
			Cid:     rp.Cid(),
			RemPath: rp.Remainder(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ResolveOutput) error {
			var (
				enc cidenc.Encoder
				err error
			)
			switch {
			case !cmdenv.CidBaseDefined(req):
				// Not specified, check the path.
				enc, err = cmdenv.CidEncoderFromPath(req.Arguments[0])
				if err == nil {
					break
				}
				// Nope, fallback on the default.
				fallthrough
			default:
				enc, err = cmdenv.GetLowLevelCidEncoder(req)
				if err != nil {
					return err
				}
			}
			p := enc.Encode(out.Cid)
			if out.RemPath != "" {
				p = ipfspath.Join([]string{p, out.RemPath})
			}

			fmt.Fprint(w, p)
			return nil
		}),
	},
	Type: ResolveOutput{},
}

type expectedRootsSeen map[cid.Cid]bool
type importResult struct {
	roots *expectedRootsSeen
	err   error
}

var DagImportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Import the contents of .car files",
		ShortDescription: `
'ipfs dag import' parses .car files and adds all objects to the blockstore.
By default, after all car files have been processed, an attempt is made to
pin each individual root specified in the car headers, before GC runs again.
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("path", true, true, "The path of a .car file.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(progressOptionName, "p", "Display progress on CLI.").WithDefault(true),
		cmds.BoolOption(silentOptionName, "No output."),
		cmds.BoolOption(pinRootsOptionName, "Pin optional roots listed in the .car headers after importing.").WithDefault(true),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		if silent, _ := req.Options[silentOptionName].(bool); silent {
			req.Options[progressOptionName] = false
		}

		return nil
	},
	Type: CarImportOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// on import ensure we do not reach out to the network for any reason
		// if a pin based on what is imported + what is in the lockstore
		// isn't possible: tough luck
		api, _ = api.WithOptions(options.Api.Offline(true))

		// grab a gc lock so that regardless of the size of the streamed-in cars
		// nothing will disappear on us before we had a chance to pin the roots
		// at the very end
		// This is especially important for use cases like dagger:
		//    ipfs dag import $( ... | ipfs-dagger --stdout=carfifos )
		//
		// !!!NOTE!!! - We take .GCLock, *not* .PinLock. The way the current stack is
		// built makes it impossible to take both a GC and a Pinning lock, regardless
		// of order. Doing so can potentially deadlock if another thread starts GC
		// ( this is not theoretical - it's been reliably simulated with the insertion
		// of strategic sleeps()s and hand-triggering GC )
		//
		// See api.Pin().Add() vs node.Pinning.Pin() comment further down
		unlocker := node.Blockstore.GCLock()
		defer func() { unlocker.Unlock() }()

		doPinRoots, _ := req.Options[pinRootsOptionName].(bool)
		showProgress, _ := req.Options[progressOptionName].(bool)

		retCh := make(chan importResult, 1)
		var counts ObjectCounts
		go importWorker(req, &res, &api, &counts, retCh)

		var roots *expectedRootsSeen

		if !showProgress {
			done := <-retCh
			if done.err != nil {
				return done.err
			}
			roots = done.roots
		} else {

			progressTicker := time.NewTicker(500 * time.Millisecond)
			defer progressTicker.Stop()

		ImportLoop:
			for {
				select {
				case ret := <-retCh:
					if ret.err != nil {
						return ret.err
					}
					roots = ret.roots

					progressTicker.Stop()
					if err := res.Emit(&CarImportOutput{ObjectCounts: counts}); err != nil {
						return err
					}
					os.Stderr.WriteString("\n")

					break ImportLoop
				case <-progressTicker.C:
					if err := res.Emit(&CarImportOutput{ObjectCounts: counts}); err != nil {
						return err
					}
				case <-req.Context.Done():
					return req.Context.Err()
				}
			}
		}

		// opportunistic pinning: try whatever sticks
		if doPinRoots {

			for c, seen := range *roots {

				// We need to re-retrieve a block, convert it to ipld, and feed it
				// to the Pinning interface, sigh...
				//
				// If we didn't have the deadlocking-gclock/pinlock problem we could
				// use the Api directly like so (though internally it does the same):
				//
				// // not ideal, but the pinning api takes only paths :(
				// rp := path.NewResolvedPath(
				// 	ipfspath.FromCid(c),
				// 	c,
				// 	c,
				// 	"",
				// )
				//
				// if err := api.Pin().Add(req.Context, rp, options.Pin.Recursive(true)); err != nil {

				ret := RootMeta{Cid: &c, PresentInCar: seen}

				if block, err := node.Blockstore.Get(c); err != nil {
					ret.PinErrorMsg = err.Error()
				} else if nd, err := ipld.Decode(block); err != nil {
					ret.PinErrorMsg = err.Error()
				} else if err := node.Pinning.Pin(req.Context, nd, true); err != nil {
					ret.PinErrorMsg = err.Error()
				} else if err := node.Pinning.Flush(req.Context); err != nil {
					ret.PinErrorMsg = err.Error()
				}

				if err := res.Emit(&CarImportOutput{Root: ret}); err != nil {
					return err
				}
			}
		}

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, event *CarImportOutput) error {

			silent, _ := req.Options[silentOptionName].(bool)
			if silent {
				return nil
			}

			// this is a progress display event
			if event.Root.Cid == nil {
				_, err := fmt.Fprintf(
					w,
					"Objects/DeclaredRoots/FoundRoots: %d/%d/%d\r",
					event.ObjectCounts.Blocks,
					event.ObjectCounts.RootsDeclared,
					event.ObjectCounts.RootsSeen,
				)
				return err
			}

			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}

			var notInCar string
			if !event.Root.PresentInCar {
				notInCar = " (specified in header without its data)"
			}

			if event.Root.PinErrorMsg != "" {
				event.Root.PinErrorMsg = fmt.Sprintf("FAILED: %s", event.Root.PinErrorMsg)
			} else {
				event.Root.PinErrorMsg = ("success")
			}

			_, err = fmt.Fprintf(
				w,
				"Pinned root %s%s: %s\n",
				enc.Encode(*event.Root.Cid),
				notInCar,
				event.Root.PinErrorMsg,
			)
			return err
		}),
	},
}

func importWorker(req *cmds.Request, re *cmds.ResponseEmitter, api *iface.CoreAPI, counts *ObjectCounts, ret chan importResult) {

	// this is *not* a transaction
	// it is simply a way to relieve pressure on the blockstore
	// similar to pinner.Pin/pinner.Flush
	batch := ipld.NewBatch(req.Context, (*api).Dag())

	// It is not guaranteed that a root in a header is actually present in the same ( or any )
	// .car file. This is the case in version 1, and ideally in further versions too
	// Accumulate any root CID seen in a header, and supplement its actual node if/when encountered
	// We will attempt a pin *only* at the end in case all car files were well formed
	//
	// The boolean value indicates whether we have encountered the root within the car file's
	expectedRootsSeen := make(expectedRootsSeen)

	it := req.Files.Entries()
	for it.Next() {

		file := files.FileFromEntry(it)
		if file == nil {
			ret <- importResult{err: errors.New("expected a file handle")}
			return
		}
		defer func() { file.Close() }()

		car, err := gocar.NewCarReader(file)
		if err != nil {
			ret <- importResult{err: err}
			return
		}

		// Be explicit here, until the spec is finished
		if car.Header.Version != 1 {
			ret <- importResult{err: errors.New("only car files version 1 supported at present")}
			return
		}

		for _, c := range car.Header.Roots {
			if _, exists := expectedRootsSeen[c]; !exists {
				expectedRootsSeen[c] = false
				counts.RootsDeclared++
			}
		}

		for {
			block, err := car.Next()
			if err != nil && err != io.EOF {
				ret <- importResult{err: err}
				return
			} else if block == nil {
				break
			}

			// the double-decode is suboptimal, but we need it for batching
			nd, err := ipld.Decode(block)
			if err != nil {
				ret <- importResult{err: err}
				return
			}

			if err := batch.Add(req.Context, nd); err != nil {
				ret <- importResult{err: err}
				return
			}

			counts.Blocks++

			// encountered something known to be a root, for the first time
			if seen, exists := expectedRootsSeen[nd.Cid()]; exists && !seen {
				expectedRootsSeen[nd.Cid()] = true
				counts.RootsSeen++
			}
		}
	}

	if err := it.Err(); err != nil {
		ret <- importResult{err: err}
		return
	}

	if err := batch.Commit(); err != nil {
		ret <- importResult{err: err}
		return
	}

	ret <- importResult{roots: &expectedRootsSeen}
}

var DagExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Streams the selected DAG as a .car stream on stdout.",
		ShortDescription: `
'ipfs dag export' fetches a dag and streams it out as a well-formed .car file.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("selector", true, false, "Selector representing the dag to export").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(progressOptionName, "p", "Display progress on CLI.").WithDefault(true),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return fmt.Errorf(
				"unable to parse selector (currently only bare CIDs are supported): %s",
				err,
			)
		}

		// The current interface of go-car is rather suboptimal as it
		// only takes a blockstore, instead of accepting a dagservice,
		// and leveraging parallel-fetch capabilities
		//
		// Until the above is fixed, pre-warm the blockstore before doing
		// anything else. We explicitly *DO NOT* take a lock during this
		// operation: even if we lose some of the blocks we just received
		// due to a conflicting GC: we will just re-retrieve everything when
		// the car is being streamed out
		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if err := mdag.FetchGraph(req.Context, c, node.DAG); err != nil {
			if !node.IsOnline {
				err = fmt.Errorf("%s (currently offline, perhaps retry after attaching to the network)", err)
			}
			return err
		}

		sess := blockservice.NewSession(
			req.Context,
			node.Blocks,
		)

		var nodeCount int64

		// The second part of the above - make a super-thin wrapper around
		// a blockservice session, translating Session.GetBlock() to Blockstore.Get()
		// The function also doubles as our progress meter
		var wrapper getBlockFromSessionWrapper = func(c cid.Cid) (blk.Block, error) {
			nodeCount++
			return sess.GetBlock(req.Context, c)
		}
		sb := gipselectorbuilder.NewSelectorSpecBuilder(gipfree.NodeBuilder())
		car := gocar.NewSelectiveCar(
			req.Context,
			&wrapper,
			[]gocar.Dag{gocar.Dag{
				Root: c,
				Selector: sb.ExploreRecursive(
					gipselector.RecursionLimitNone(),
					sb.ExploreAll(sb.ExploreRecursiveEdge()),
				).Node(),
			}},
		)

		if showProgress, _ := req.Options[progressOptionName].(bool); showProgress {
			stopProgress := make(chan bool)
			progressTicker := time.NewTicker(500 * time.Millisecond)
			defer func() { stopProgress <- true }()

			go func() {
				emitProgress := func() error {
					_, err := fmt.Fprintf(
						os.Stderr,
						"Exported objects: %d\r",
						nodeCount,
					)
					return err
				}

				for {
					select {
					case <-stopProgress:
						progressTicker.Stop()
						_ = emitProgress()
						os.Stderr.WriteString("\n")
						return
					case <-progressTicker.C:
						if emitProgress() != nil {
							return
						}
					}
				}
			}()
		}

		pipeR, pipeW := io.Pipe()

		errCh := make(chan error, 2) // we only report the 1st error
		go func() {
			defer func() {
				if err := pipeW.Close(); err != nil {
					errCh <- fmt.Errorf("stream flush failed: %s", err)
				}
				close(errCh)
			}()

			if err := car.Write(pipeW); err != nil {
				errCh <- err
			}
		}()

		if err := res.Emit(pipeR); err != nil {
			pipeW.Close() // ignore errors if any
			return err
		}

		if err := <-errCh; err != nil {
			return err
		}

		return nil
	},
}

type getBlockFromSessionWrapper func(cid.Cid) (blk.Block, error)

func (w *getBlockFromSessionWrapper) Get(c cid.Cid) (blk.Block, error) {
	return (*w)(c)
}
