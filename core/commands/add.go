package commands

import (
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/cheggaaa/pb"
	"github.com/ipfs/go-ipfs/core/coreunix"

	cmds "github.com/ipfs/go-ipfs/commands"
	files "github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	u "github.com/ipfs/go-ipfs/util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

const (
	quietOptionName    = "quiet"
	progressOptionName = "progress"
	trickleOptionName  = "trickle"
	wrapOptionName     = "wrap-with-directory"
	hiddenOptionName   = "hidden"
	onlyHashOptionName = "only-hash"
	chunkerOptionName  = "chunker"
)

var AddCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add an object to ipfs.",
		ShortDescription: `
Adds contents of <path> to ipfs. Use -r to add directories.
Note that directories are added recursively, to form the ipfs
MerkleDAG. A smarter partial add with a staging area (like git)
remains to be implemented.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("path", true, true, "The path to a file to be added to IPFS").EnableRecursive().EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.OptionRecursivePath, // a builtin option that allows recursive paths (-r, --recursive)
		cmds.BoolOption(quietOptionName, "q", "Write minimal output"),
		cmds.BoolOption(progressOptionName, "p", "Stream progress data"),
		cmds.BoolOption(trickleOptionName, "t", "Use trickle-dag format for dag generation"),
		cmds.BoolOption(onlyHashOptionName, "n", "Only chunk and hash - do not write to disk"),
		cmds.BoolOption(wrapOptionName, "w", "Wrap files with a directory object"),
		cmds.BoolOption(hiddenOptionName, "H", "Include files that are hidden"),
		cmds.StringOption(chunkerOptionName, "s", "chunking algorithm to use"),
	},
	PreRun: func(req cmds.Request) error {
		if quiet, _, _ := req.Option(quietOptionName).Bool(); quiet {
			return nil
		}

		req.SetOption(progressOptionName, true)

		sizeFile, ok := req.Files().(files.SizeFile)
		if !ok {
			// we don't need to error, the progress bar just won't know how big the files are
			return nil
		}

		size, err := sizeFile.Size()
		if err != nil {
			// see comment above
			return nil
		}

		log.Debugf("Total size of file being added: %v\n", size)
		req.Values()["size"] = size

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
		chunker, _, _ := req.Option(chunkerOptionName).String()

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

		fileAdder := coreunix.NewAdder(req.Context(), n, outChan)
		fileAdder.Chunker = chunker
		fileAdder.Progress = progress
		fileAdder.Hidden = hidden
		fileAdder.Trickle = trickle
		fileAdder.Wrap = wrap

		// addAllFiles loops over a convenience slice file to
		// add each file individually. e.g. 'ipfs add a b c'
		addAllFiles := func(sliceFile files.File) error {
			for {
				file, err := sliceFile.NextFile()
				if err != nil && err != io.EOF {
					return err
				}
				if file == nil {
					return nil // done
				}

				if _, err := fileAdder.AddFile(file); err != nil {
					return err
				}
			}
		}

		addAllAndPin := func(f files.File) error {
			if err := addAllFiles(f); err != nil {
				return err
			}

			if !hash {
				// copy intermediary nodes from editor to our actual dagservice
				err := fileAdder.WriteOutputTo(n.DAG)
				if err != nil {
					log.Error("WRITE OUT: ", err)
					return err
				}
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

		size := int64(0)
		s, found := req.Values()["size"]
		if found {
			size = s.(int64)
		}
		showProgressBar := !quiet && size >= progressBarMinSize

		var bar *pb.ProgressBar
		var terminalWidth int
		if showProgressBar {
			bar = pb.New64(size).SetUnits(pb.U_BYTES)
			bar.ManualUpdate = true
			bar.Start()

			// the progress bar lib doesn't give us a way to get the width of the output,
			// so as a hack we just use a callback to measure the output, then git rid of it
			terminalWidth = 0
			bar.Callback = func(line string) {
				terminalWidth = len(line)
				bar.Callback = nil
				bar.Output = res.Stderr()
				log.Infof("terminal width: %v\n", terminalWidth)
			}
			bar.Update()
		}

		lastFile := ""
		var totalProgress, prevFiles, lastBytes int64

		for out := range outChan {
			output := out.(*coreunix.AddedObject)
			if len(output.Hash) > 0 {
				if showProgressBar {
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

				if !showProgressBar {
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

			if showProgressBar {
				bar.Update()
			}
		}
	},
	Type: coreunix.AddedObject{},
}
