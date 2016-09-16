package commands

import (
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/filestore/support"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"gx/ipfs/QmeWjRodbcZFKe5tMN7poEx3izym6osrLSnTLf9UjJZBbs/pb"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	cmds "github.com/ipfs/go-ipfs/commands"
	files "github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	dagtest "github.com/ipfs/go-ipfs/merkledag/test"
	mfs "github.com/ipfs/go-ipfs/mfs"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

const (
	quietOptionName    = "quiet"
	silentOptionName   = "silent"
	progressOptionName = "progress"
	trickleOptionName  = "trickle"
	wrapOptionName     = "wrap-with-directory"
	hiddenOptionName   = "hidden"
	onlyHashOptionName = "only-hash"
	chunkerOptionName  = "chunker"
	pinOptionName      = "pin"
	allowDupName       = "allow-dup"
)

var AddCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a file to ipfs.",
		ShortDescription: `
Adds contents of <path> to ipfs. Use -r to add directories.
Note that directories are added recursively, to form the ipfs
MerkleDAG.
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
		cmds.FileArg("path", true, true, "The path to a file to be added to IPFS.").EnableRecursive().EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.OptionRecursivePath, // a builtin option that allows recursive paths (-r, --recursive)
		cmds.BoolOption(quietOptionName, "q", "Write minimal output.").Default(false),
		cmds.BoolOption(silentOptionName, "Write no output.").Default(false),
		cmds.BoolOption(progressOptionName, "p", "Stream progress data."),
		cmds.BoolOption(trickleOptionName, "t", "Use trickle-dag format for dag generation.").Default(false),
		cmds.BoolOption(onlyHashOptionName, "n", "Only chunk and hash - do not write to disk.").Default(false),
		cmds.BoolOption(wrapOptionName, "w", "Wrap files with a directory object.").Default(false),
		cmds.BoolOption(hiddenOptionName, "H", "Include files that are hidden. Only takes effect on recursive add.").Default(false),
		cmds.StringOption(chunkerOptionName, "s", "Chunking algorithm to use."),
		cmds.BoolOption(pinOptionName, "Pin this object when adding.").Default(true),
		cmds.BoolOption(allowDupName, "Add even if blocks are in non-cache blockstore.").Default(false),
	},
	PreRun: func(req cmds.Request) error {
		if quiet, _, _ := req.Option(quietOptionName).Bool(); quiet {
			return nil
		}

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
		recursive, _, _ := req.Option(cmds.RecLong).Bool()
		allowDup, _, _ := req.Option(allowDupName).Bool()

		nocopy, _ := req.Values()["no-copy"].(bool)

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

		outChan := make(chan interface{}, 8)
		res.SetOutput((<-chan interface{})(outChan))

		var fileAdder *coreunix.Adder
		useRoot := wrap || recursive
		perFileLocker := filestore.NoOpLocker()
		if nocopy {
			fs, ok := n.Repo.DirectMount(fsrepo.FilestoreMount).(*filestore.Datastore)
			if !ok {
				res.SetError(errors.New("filestore not enabled"), cmds.ErrNormal)
				return
			}
			blockstore := filestore_support.NewBlockstore(n.Blockstore, fs)
			blockService := bserv.New(blockstore, n.Exchange)
			dagService := dag.NewDAGService(blockService)
			fileAdder, err = coreunix.NewAdder(req.Context(), n.Pinning, blockstore, dagService, useRoot)
			fileAdder.FullName = true
			perFileLocker = fs.AddLocker()
		} else if allowDup {
			// add directly to the first mount bypassing
			// the Has() check of the multi-blockstore
			blockstore := bs.NewGCBlockstore(n.Blockstore.FirstMount(), n.Blockstore)
			blockService := bserv.New(blockstore, n.Exchange)
			dagService := dag.NewDAGService(blockService)
			fileAdder, err = coreunix.NewAdder(req.Context(), n.Pinning, blockstore, dagService, useRoot)
		} else {
			fileAdder, err = coreunix.NewAdder(req.Context(), n.Pinning, n.Blockstore, n.DAG, useRoot)
		}
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

		if hash {
			md := dagtest.Mock()
			mr, err := mfs.NewRoot(req.Context(), md, ft.EmptyDirNode(), nil)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			fileAdder.SetMfsRoot(mr)
		}

		addAllAndPin := func(f files.File) {
			// Iterate over each top-level file and add individually. Otherwise the
			// single files.File f is treated as a directory, affecting hidden file
			// semantics.
			for {
				file, err := f.NextFile()
				if err == io.EOF {
					// Finished the list of files.
					break
				} else if err != nil {
					outChan <- &coreunix.AddedObject{Name: f.FullPath(), Error: err.Error()}
					return
				}
				perFileLocker.Lock()
				defer perFileLocker.Unlock()
				if err := fileAdder.AddFile(file); err != nil {
					outChan <- &coreunix.AddedObject{Name: f.FullPath(), Error: err.Error()}
					return
				}
			}

			// copy intermediary nodes from editor to our actual dagservice
			_, err := fileAdder.Finalize()
			if err != nil {
				outChan <- &coreunix.AddedObject{Error: err.Error()}
				return
			}

			if hash {
				return
			}

			err = fileAdder.PinRoot()
			if err != nil {
				outChan <- &coreunix.AddedObject{Error: err.Error()}
				return
			}
		}

		go func() {
			defer close(outChan)
			addAllAndPin(req.Files())
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

		silent, _, err := req.Option(silentOptionName).Bool()
		if err != nil {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}

		if !quiet && !silent {
			progress = true
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
				if len(output.Error) > 0 {
					res.SetError(errors.New(output.Error), cmds.ErrNormal)
					return
				} else if len(output.Hash) > 0 {
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
