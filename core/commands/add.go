package commands

import (
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/cheggaaa/pb"

	cmds "github.com/ipfs/go-ipfs/commands"
	files "github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	importer "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

// how many bytes of progress to wait before sending a progress update message
const progressReaderIncrement = 1024 * 256

const (
	progressOptionName = "progress"
	wrapOptionName     = "wrap-with-directory"
)

type AddedObject struct {
	Name  string
	Hash  string `json:",omitempty"`
	Bytes int64  `json:",omitempty"`
}

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
		cmds.BoolOption("quiet", "q", "Write minimal output"),
		cmds.BoolOption(progressOptionName, "p", "Stream progress data"),
		cmds.BoolOption(wrapOptionName, "w", "Wrap files with a directory object"),
		cmds.BoolOption("t", "trickle", "Use trickle-dag format for dag generation"),
	},
	PreRun: func(req cmds.Request) error {
		if quiet, _, _ := req.Option("quiet").Bool(); quiet {
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
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		progress, _, _ := req.Option(progressOptionName).Bool()
		wrap, _, _ := req.Option(wrapOptionName).Bool()

		outChan := make(chan interface{}, 8)
		res.SetOutput((<-chan interface{})(outChan))

		go func() {
			defer close(outChan)

			for {
				file, err := req.Files().NextFile()
				if err != nil && err != io.EOF {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				if file == nil { // done
					return
				}

				rootnd, err := addFile(n, file, outChan, progress, wrap)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				rnk, err := rootnd.Key()
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				mp := n.Pinning.GetManual()
				mp.RemovePinWithMode(rnk, pin.Indirect)
				mp.PinWithMode(rnk, pin.Recursive)

				err = n.Pinning.Flush()
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
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
			output := out.(*AddedObject)
			if len(output.Hash) > 0 {
				if showProgressBar {
					// clear progress bar line before we print "added x" output
					fmt.Fprintf(res.Stderr(), "\r%s\r", strings.Repeat(" ", terminalWidth))
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
	Type: AddedObject{},
}

func add(n *core.IpfsNode, reader io.Reader) (*dag.Node, error) {
	node, err := importer.BuildDagFromReader(
		reader,
		n.DAG,
		chunk.DefaultSplitter,
		importer.PinIndirectCB(n.Pinning.GetManual()),
	)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func addFile(n *core.IpfsNode, file files.File, out chan interface{}, progress bool, wrap bool) (*dag.Node, error) {
	if file.IsDirectory() {
		return addDir(n, file, out, progress)
	}

	// if the progress flag was specified, wrap the file so that we can send
	// progress updates to the client (over the output channel)
	var reader io.Reader = file
	if progress {
		reader = &progressReader{file: file, out: out}
	}

	if wrap {
		p, dagnode, err := coreunix.AddWrapped(n, reader, path.Base(file.FileName()))
		if err != nil {
			return nil, err
		}
		out <- &AddedObject{
			Hash: p,
			Name: file.FileName(),
		}
		return dagnode, nil
	}

	dagnode, err := add(n, reader)
	if err != nil {
		return nil, err
	}

	log.Infof("adding file: %s", file.FileName())
	if err := outputDagnode(out, file.FileName(), dagnode); err != nil {
		return nil, err
	}
	return dagnode, nil
}

func addDir(n *core.IpfsNode, dir files.File, out chan interface{}, progress bool) (*dag.Node, error) {
	log.Infof("adding directory: %s", dir.FileName())

	tree := &dag.Node{Data: ft.FolderPBData()}

	for {
		file, err := dir.NextFile()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if file == nil {
			break
		}

		node, err := addFile(n, file, out, progress, false)
		if err != nil {
			return nil, err
		}

		_, name := path.Split(file.FileName())

		err = tree.AddNodeLink(name, node)
		if err != nil {
			return nil, err
		}
	}

	err := outputDagnode(out, dir.FileName(), tree)
	if err != nil {
		return nil, err
	}

	k, err := n.DAG.Add(tree)
	if err != nil {
		return nil, err
	}

	n.Pinning.GetManual().PinWithMode(k, pin.Indirect)

	return tree, nil
}

// outputDagnode sends dagnode info over the output channel
func outputDagnode(out chan interface{}, name string, dn *dag.Node) error {
	o, err := getOutput(dn)
	if err != nil {
		return err
	}

	out <- &AddedObject{
		Hash: o.Hash,
		Name: name,
	}

	return nil
}

type progressReader struct {
	file         files.File
	out          chan interface{}
	bytes        int64
	lastProgress int64
}

func (i *progressReader) Read(p []byte) (int, error) {
	n, err := i.file.Read(p)

	i.bytes += int64(n)
	if i.bytes-i.lastProgress >= progressReaderIncrement || err == io.EOF {
		i.lastProgress = i.bytes
		i.out <- &AddedObject{
			Name:  i.file.FileName(),
			Bytes: i.bytes,
		}
	}

	return n, err
}
