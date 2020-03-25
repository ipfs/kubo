package dagcmd

import (
	"bytes"
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
	//gipfree "github.com/ipld/go-ipld-prime/impl/free"
	//gipselector "github.com/ipld/go-ipld-prime/traversal/selector"
	//gipselectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"
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
	ObjectCounts *ObjectCounts `json:",omitempty"`
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

// It is not guaranteed that a root in a header is actually present in the same ( or any )
// .car file. This is the case in version 1, and ideally in further versions too
// Accumulate any root CID seen in a header, and supplement its actual node if/when encountered
// We will attempt a pin *only* at the end in case all car files were well formed
//
// The boolean value indicates whether we have encountered the root within the car file's
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
		cmds.BoolOption(progressOptionName, "p", "Display progress on CLI. Defaults to true when STDERR is a TTY"),
		cmds.BoolOption(silentOptionName, "No output."),
		cmds.BoolOption(pinRootsOptionName, "Pin optional roots listed in the .car headers after importing.").WithDefault(true),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		silent, _ := req.Options[silentOptionName].(bool)
		encType, _ := req.Options[cmds.EncLong].(string)

		if silent || encType != "text" {
			// force-disable progress unless on non-silent CLI
			req.Options[progressOptionName] = false
		} else if _, specified := req.Options[progressOptionName]; !specified {
			// enable progress implicitly if a TTY
			errStat, _ := os.Stderr.Stat()
			if 0 != (errStat.Mode() & os.ModeCharDevice) {
				req.Options[progressOptionName] = true
			}
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
		unlocker := node.Blockstore.PinLock()
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
					if err := res.Emit(&CarImportOutput{ObjectCounts: &counts}); err != nil {
						return err
					}

					// -1 is signal to emit the final newline
					if err := res.Emit(&CarImportOutput{ObjectCounts: &ObjectCounts{Blocks: -1}}); err != nil {
						return err
					}

					break ImportLoop
				case <-progressTicker.C:
					if err := res.Emit(&CarImportOutput{ObjectCounts: &counts}); err != nil {
						return err
					}
				case <-req.Context.Done():
					return req.Context.Err()
				}
			}
		}

		// opportunistic pinning: try whatever sticks
		if doPinRoots {

			var failedPins int
			for c, seen := range *roots {

				// We need to re-retrieve a block, convert it to ipld, and feed it
				// to the Pinning interface, sigh...
				//
				// If we didn't have the problem of inability to take multiple pinlocks,
				// we could use the Api directly like so (though internally it does the same):
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

				if ret.PinErrorMsg != "" {
					failedPins++
				}

				if err := res.Emit(&CarImportOutput{Root: ret}); err != nil {
					return err
				}
			}

			if failedPins > 0 {
				return fmt.Errorf(
					"unable to pin all roots: %d out of %d failed",
					failedPins,
					len(*roots),
				)
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

				var printErr error

				// -1 means we are done: just print a "\n"
				if event.ObjectCounts.Blocks < 0 {
					_, printErr = w.Write([]byte("\n"))
				} else {
					_, printErr = fmt.Fprintf(
						w,
						"Objects/DeclaredRoots/ImportedRoots:\t%d/%d/%d\r",
						event.ObjectCounts.Blocks,
						event.ObjectCounts.RootsDeclared,
						event.ObjectCounts.RootsSeen,
					)
				}

				return printErr
			}

			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}

			if event.Root.PinErrorMsg != "" {
				event.Root.PinErrorMsg = fmt.Sprintf("FAILED: %s", event.Root.PinErrorMsg)
			} else {
				event.Root.PinErrorMsg = "success"
			}

			if !event.Root.PresentInCar {
				event.Root.PinErrorMsg += " (root specified in .car header without its data)"
			}

			_, err = fmt.Fprintf(
				w,
				"Pinned root\t%s\t%s\n",
				enc.Encode(*event.Root.Cid),
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

	roots := make(expectedRootsSeen)

	it := req.Files.Entries()
	for it.Next() {

		file := files.FileFromEntry(it)
		if file == nil {
			ret <- importResult{err: errors.New("expected a file handle")}
			return
		}

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
			if _, exists := roots[c]; !exists {
				roots[c] = false
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
			if seen, exists := roots[nd.Cid()]; exists && !seen {
				roots[nd.Cid()] = true
				counts.RootsSeen++
			}
		}

		// every single file in it. is already open before we start
		// just close here sooner rather than later for neatness
		// this won't/can't help with not running out of handles
		file.Close()
	}

	if err := it.Err(); err != nil {
		ret <- importResult{err: err}
		return
	}

	if err := batch.Commit(); err != nil {
		ret <- importResult{err: err}
		return
	}

	ret <- importResult{roots: &roots}
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
		cmds.BoolOption(progressOptionName, "p", "Display progress on CLI. Defaults to true when STDERR is a TTY."),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		// enable progress implicitly if a TTY
		if _, specified := req.Options[progressOptionName]; !specified {
			errStat, _ := os.Stderr.Stat()
			if 0 != (errStat.Mode() & os.ModeCharDevice) {
				req.Options[progressOptionName] = true
			}
		}

		return nil
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
		// due to a conflicting GC: we will just re-retrieve anything we
		// potentially lost when the car is being streamed out
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

		// The second part of the above - make a super-thin wrapper around
		// a blockservice session, translating Session.GetBlock() to Blockstore.Get()
		//
		// sess := blockservice.NewSession(
		// 	req.Context,
		// 	node.Blocks,
		// )
		// var wrapper getBlockFromSessionWrapper = func(c cid.Cid) (blk.Block, error) {
		// 	return sess.GetBlock(req.Context, c)
		// }
		// sb := gipselectorbuilder.NewSelectorSpecBuilder(gipfree.NodeBuilder())
		// car := gocar.NewSelectiveCar(
		// 	req.Context,
		// 	&wrapper,
		// 	[]gocar.Dag{gocar.Dag{
		// 		Root: c,
		// 		Selector: sb.ExploreRecursive(
		// 			gipselector.RecursionLimitNone(),
		// 			sb.ExploreAll(sb.ExploreRecursiveEdge()),
		// 		).Node(),
		// 	}},
		// )

		pipeR, pipeW := io.Pipe()

		errCh := make(chan error, 2) // we only report the 1st error
		go func() {
			defer func() {
				if err := pipeW.Close(); err != nil {
					errCh <- fmt.Errorf("stream flush failed: %s", err)
				}
				close(errCh)
			}()

			//if err := car.Write(pipeW); err != nil {
			if err := gocar.WriteCar(
				req.Context,
				node.DAG,
				[]cid.Cid{c},
				pipeW,
			); err != nil {
				errCh <- err
			}
		}()

		if err := res.Emit(pipeR); err != nil {
			pipeW.Close() // ignore errors if any
			return err
		}

		return <-errCh
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			if showProgress, _ := res.Request().Options[progressOptionName].(bool); !showProgress {
				return cmds.Copy(re, res)
			}

			var exportSize int64
			buf := make([]byte, 4*1024*1024)
			defer func() {
				if exportSize > 0 {
					os.Stderr.WriteString("\n")
				}
			}()

			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return re.Close()
					}
					return re.CloseWithError(err)
				}

				if r, ok := v.(io.Reader); ok {
					// we got a reader passed as a response
					// proxy it through with an increasing counter
					for {
						len, readErr := r.Read(buf)
						if len > 0 {
							if err := re.Emit(bytes.NewBuffer(buf[:len])); err != nil {
								return err
							}

							exportSize += int64(len)
							fmt.Fprintf(
								os.Stderr,
								"Exported .car size:\t%d\r",
								exportSize,
							)
						}

						if readErr == io.EOF {
							return re.Close()
						} else if readErr != nil {
							return re.CloseWithError(err)
						}
					}
				} else {
					// some sort of encoded response, just get on with it
					err := re.Emit(v)
					if err != nil {
						return err
					}
				}
			}
		},
	},
}
