package commands

import (
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs/core/coreunix"
	"gx/ipfs/QmeWjRodbcZFKe5tMN7poEx3izym6osrLSnTLf9UjJZBbs/pb"

	blockservice "github.com/ipfs/go-ipfs/blockservice"
	cmds "github.com/ipfs/go-ipfs/commands"
	files "github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
	dagtest "github.com/ipfs/go-ipfs/merkledag/test"
	mfs "github.com/ipfs/go-ipfs/mfs"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

const (
	quietOptionName     = "quiet"
	silentOptionName    = "silent"
	progressOptionName  = "progress"
	trickleOptionName   = "trickle"
	wrapOptionName      = "wrap-with-directory"
	hiddenOptionName    = "hidden"
	onlyHashOptionName  = "only-hash"
	chunkerOptionName   = "chunker"
	pinOptionName       = "pin"
	rawLeavesOptionName = "raw-leaves"
)

var AddCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a file or directory to ipfs.",
		ShortDescription: `
Adds contents of <path> to ipfs. Use -r to add directories (recursively).
`,
		LongDescription: `
Adds contents of <path> to ipfs. Use -r to add directories.
Note that directories are added recursively, to form the ipfs
MerkleDAG.

The wrap option, '-w', wraps the file (or files, if using the
recursive option) in a directory. This directory contains only
the files which have been added, and means that the file retains
its filename. For example:

  > ipfs add example.jpg
  added QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH example.jpg
  > ipfs add example.jpg -w
  added QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH example.jpg
  added QmaG4FuMqEBnQNn3C8XJ5bpW8kLs7zq2ZXgHptJHbKDDVx

You can now refer to the added file in a gateway, like so:

  /ipfs/QmaG4FuMqEBnQNn3C8XJ5bpW8kLs7zq2ZXgHptJHbKDDVx/example.jpg
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("path", true, true, "The path to a file to be added to ipfs.").EnableRecursive().EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.OptionRecursivePath, // a builtin option that allows recursive paths (-r, --recursive)
		cmds.BoolOption(quietOptionName, "q", "Write minimal output."),
		cmds.BoolOption(silentOptionName, "Write no output."),
		cmds.BoolOption(progressOptionName, "p", "Stream progress data."),
		cmds.BoolOption(trickleOptionName, "t", "Use trickle-dag format for dag generation."),
		cmds.BoolOption(onlyHashOptionName, "n", "Only chunk and hash - do not write to disk."),
		cmds.BoolOption(wrapOptionName, "w", "Wrap files with a directory object."),
		cmds.BoolOption(hiddenOptionName, "H", "Include files that are hidden. Only takes effect on recursive add."),
		cmds.StringOption(chunkerOptionName, "s", "Chunking algorithm to use."),
		cmds.BoolOption(pinOptionName, "Pin this object when adding.").Default(true),
		cmds.BoolOption(rawLeavesOptionName, "Use raw blocks for leaf nodes. (experimental)"),
	},
	PreRun: func(req cmds.Request) error {
		quiet, _, _ := req.Option(quietOptionName).Bool()
		silent, _, _ := req.Option(silentOptionName).Bool()

		if quiet || silent {
			return nil
		}

		// ipfs cli progress bar defaults to true unless quiet or silent is used
		_, found, _ := req.Option(progressOptionName).Bool()
		if !found {
			req.SetOption(progressOptionName, true)
		}

		sizeFile, ok := req.Files().(files.SizeFile)
		if !ok {
			// we don't need to error, the progress bar just won't know how big the files are
			log.Warning("cannnot determine size of input file")
			return nil
		}

		sizeCh := make(chan int64, 1)
		req.Values()["size"] = sizeCh

		go func() {
			size, err := sizeFile.Size()
			if err != nil {
				log.Warningf("error getting files size: %s", err)
				// see comment above
				return
			}

			log.Debugf("Total size of file being added: %v\n", size)
			sizeCh <- size
		}()

		return nil
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		// check if repo will exceed storage limit if added
		// TODO: this doesn't handle the case if the hashed file is already in blocks (deduplicated)
		// TODO: conditional GC is disabled due to it is somehow not possible to pass the size to the daemon
		//if err := corerepo.ConditionalGC(req.Context(), n, uint64(size)); err != nil {
		//	res.SetError(err, cmds.ErrNormal)
		//	return
		//}

		progress, _, _ := req.Option(progressOptionName).Bool()
		trickle, _, _ := req.Option(trickleOptionName).Bool()
		wrap, _, _ := req.Option(wrapOptionName).Bool()
		hash, _, _ := req.Option(onlyHashOptionName).Bool()
		hidden, _, _ := req.Option(hiddenOptionName).Bool()
		silent, _, _ := req.Option(silentOptionName).Bool()
		chunker, _, _ := req.Option(chunkerOptionName).String()
		dopin, _, _ := req.Option(pinOptionName).Bool()
		rawblks, _, _ := req.Option(rawLeavesOptionName).Bool()

		if hash {
			nilnode, err := core.NewNode(n.Context(), &core.BuildCfg{
				//TODO: need this to be true or all files
				// hashed will be stored in memory!
				NilRepo: true,
			})
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			n = nilnode
		}

		dserv := n.DAG
		local, _, _ := req.Option("local").Bool()
		if local {
			offlineexch := offline.Exchange(n.Blockstore)
			bserv := blockservice.New(n.Blockstore, offlineexch)
			dserv = dag.NewDAGService(bserv)
		}

		outChan := make(chan interface{}, 8)
		res.SetOutput((<-chan interface{})(outChan))

		fileAdder, err := coreunix.NewAdder(req.Context(), n.Pinning, n.Blockstore, dserv)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fileAdder.Out = outChan
		fileAdder.Chunker = chunker
		fileAdder.Progress = progress
		fileAdder.Hidden = hidden
		fileAdder.Trickle = trickle
		fileAdder.Wrap = wrap
		fileAdder.Pin = dopin
		fileAdder.Silent = silent
		fileAdder.RawLeaves = rawblks

		if hash {
			md := dagtest.Mock()
			mr, err := mfs.NewRoot(req.Context(), md, ft.EmptyDirNode(), nil)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			fileAdder.SetMfsRoot(mr)
		}

		addAllAndPin := func(f files.File) error {
			// Iterate over each top-level file and add individually. Otherwise the
			// single files.File f is treated as a directory, affecting hidden file
			// semantics.
			for {
				file, err := f.NextFile()
				if err == io.EOF {
					// Finished the list of files.
					break
				} else if err != nil {
					return err
				}
				if err := fileAdder.AddFile(file); err != nil {
					return err
				}
			}

			// copy intermediary nodes from editor to our actual dagservice
			_, err := fileAdder.Finalize()
			if err != nil {
				return err
			}

			if hash {
				return nil
			}

			return fileAdder.PinRoot()
		}

		go func() {
			defer close(outChan)
			if err := addAllAndPin(req.Files()); err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

		}()
	},
	PostRun: func(req cmds.Request, res cmds.Response) {
		if res.Error() != nil {
			return
		}
		outChan, ok := res.Output().(<-chan interface{})
		if !ok {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}
		res.SetOutput(nil)

		quiet, _, err := req.Option("quiet").Bool()
		if err != nil {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}

		progress, _, err := req.Option(progressOptionName).Bool()
		if err != nil {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}

		var bar *pb.ProgressBar
		if progress {
			bar = pb.New64(0).SetUnits(pb.U_BYTES)
			bar.ManualUpdate = true
			bar.ShowTimeLeft = false
			bar.ShowPercent = false
			bar.Output = res.Stderr()
			bar.Start()
		}

		var sizeChan chan int64
		s, found := req.Values()["size"]
		if found {
			sizeChan = s.(chan int64)
		}

		lastFile := ""
		var totalProgress, prevFiles, lastBytes int64

	LOOP:
		for {
			select {
			case out, ok := <-outChan:
				if !ok {
					break LOOP
				}
				output := out.(*coreunix.AddedObject)
				if len(output.Hash) > 0 {
					if progress {
						// clear progress bar line before we print "added x" output
						fmt.Fprintf(res.Stderr(), "\033[2K\r")
					}
					if quiet {
						fmt.Fprintf(res.Stdout(), "%s\n", output.Hash)
					} else {
						fmt.Fprintf(res.Stdout(), "added %s %s\n", output.Hash, output.Name)
					}

				} else {
					log.Debugf("add progress: %v %v\n", output.Name, output.Bytes)

					if !progress {
						continue
					}

					if len(lastFile) == 0 {
						lastFile = output.Name
					}
					if output.Name != lastFile || output.Bytes < lastBytes {
						prevFiles += lastBytes
						lastFile = output.Name
					}
					lastBytes = output.Bytes
					delta := prevFiles + lastBytes - totalProgress
					totalProgress = bar.Add64(delta)
				}

				if progress {
					bar.Update()
				}
			case size := <-sizeChan:
				if progress {
					bar.Total = size
					bar.ShowPercent = true
					bar.ShowBar = true
					bar.ShowTimeLeft = true
				}
			case <-req.Context().Done():
				res.SetError(req.Context().Err(), cmds.ErrNormal)
				return
			}
		}
	},
	Type: coreunix.AddedObject{},
}
