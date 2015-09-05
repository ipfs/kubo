package commands

import (
	"fmt"
	"io"
	"path"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/cheggaaa/pb"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	cxt "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	cmds "github.com/ipfs/go-ipfs/commands"
	files "github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	importer "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

// how many bytes of progress to wait before sending a progress update message
const progressReaderIncrement = 1024 * 256

const (
	quietOptionName    = "quiet"
	progressOptionName = "progress"
	trickleOptionName  = "trickle"
	wrapOptionName     = "wrap-with-directory"
	hiddenOptionName   = "hidden"
	onlyHashOptionName = "only-hash"
	chunkerOptionName  = "chunker"
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
		cmds.BoolOption(quietOptionName, "q", "Write minimal output"),
		cmds.BoolOption(progressOptionName, "p", "Stream progress data"),
		cmds.BoolOption(trickleOptionName, "t", "Use trickle-dag format for dag generation"),
		cmds.BoolOption(onlyHashOptionName, "n", "Only chunk and hash - do not write to disk"),
		cmds.BoolOption(wrapOptionName, "w", "Wrap files with a directory object"),
		cmds.BoolOption(hiddenOptionName, "Include files that are hidden"),
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

		progress, _, _ := req.Option(progressOptionName).Bool()
		trickle, _, _ := req.Option(trickleOptionName).Bool()
		wrap, _, _ := req.Option(wrapOptionName).Bool()
		hash, _, _ := req.Option(onlyHashOptionName).Bool()
		hidden, _, _ := req.Option(hiddenOptionName).Bool()
		chunker, _, _ := req.Option(chunkerOptionName).String()

		e := dagutils.NewDagEditor(NewMemoryDagService(), newDirNode())
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

		fileAdder := adder{
			ctx:      req.Context(),
			node:     n,
			editor:   e,
			out:      outChan,
			chunker:  chunker,
			progress: progress,
			hidden:   hidden,
			trickle:  trickle,
			wrap:     wrap,
		}

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

				if _, err := fileAdder.addFile(file); err != nil {
					return err
				}
			}
		}

		pinRoot := func(rootnd *dag.Node) error {
			rnk, err := rootnd.Key()
			if err != nil {
				return err
			}

			mp := n.Pinning.GetManual()
			mp.RemovePinWithMode(rnk, pin.Indirect)
			mp.PinWithMode(rnk, pin.Recursive)
			return n.Pinning.Flush()
		}

		addAllAndPin := func(f files.File) error {
			if err := addAllFiles(f); err != nil {
				return err
			}

			if !hash {
				// copy intermediary nodes from editor to our actual dagservice
				err := e.WriteOutputTo(n.DAG)
				if err != nil {
					log.Error("WRITE OUT: ", err)
					return err
				}
			}

			rootnd, err := fileAdder.RootNode()
			if err != nil {
				return err
			}

			return pinRoot(rootnd)
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
			output := out.(*AddedObject)
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
	Type: AddedObject{},
}

func NewMemoryDagService() dag.DAGService {
	// build mem-datastore for editor's intermediary nodes
	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	return dag.NewDAGService(bsrv)
}

// Internal structure for holding the switches passed to the `add` call
type adder struct {
	ctx      cxt.Context
	node     *core.IpfsNode
	editor   *dagutils.Editor
	out      chan interface{}
	progress bool
	hidden   bool
	trickle  bool
	wrap     bool
	chunker  string

	nextUntitled int
}

// Perform the actual add & pin locally, outputting results to reader
func add(n *core.IpfsNode, reader io.Reader, useTrickle bool, chunker string) (*dag.Node, error) {
	chnk, err := chunk.FromString(reader, chunker)
	if err != nil {
		return nil, err
	}

	var node *dag.Node
	if useTrickle {
		node, err = importer.BuildTrickleDagFromReader(
			n.DAG,
			chnk,
			importer.PinIndirectCB(n.Pinning.GetManual()),
		)
	} else {
		node, err = importer.BuildDagFromReader(
			n.DAG,
			chnk,
			importer.PinIndirectCB(n.Pinning.GetManual()),
		)
	}

	if err != nil {
		return nil, err
	}

	return node, nil
}

func (params *adder) RootNode() (*dag.Node, error) {
	r := params.editor.GetNode()

	// if not wrapping, AND one root file, use that hash as root.
	if !params.wrap && len(r.Links) == 1 {
		var err error
		r, err = r.Links[0].GetNode(params.ctx, params.editor.GetDagService())
		// no need to output, as we've already done so.
		return r, err
	}

	// otherwise need to output, as we have not.
	err := outputDagnode(params.out, "", r)
	return r, err
}

func (params *adder) addNode(node *dag.Node, path string) error {
	// patch it into the root
	if path == "" {
		key, err := node.Key()
		if err != nil {
			return err
		}

		path = key.Pretty()
	}

	if err := params.editor.InsertNodeAtPath(params.ctx, path, node, newDirNode); err != nil {
		return err
	}

	return outputDagnode(params.out, path, node)
}

// Add the given file while respecting the params.
func (params *adder) addFile(file files.File) (*dag.Node, error) {
	// Check if file is hidden
	if fileIsHidden := files.IsHidden(file); fileIsHidden && !params.hidden {
		log.Debugf("%s is hidden, skipping", file.FileName())
		return nil, &hiddenFileError{file.FileName()}
	}

	// Check if "file" is actually a directory
	if file.IsDirectory() {
		return params.addDir(file)
	}

	if s, ok := file.(*files.Symlink); ok {
		sdata, err := ft.SymlinkData(s.Target)
		if err != nil {
			return nil, err
		}

		dagnode := &dag.Node{Data: sdata}
		_, err = params.node.DAG.Add(dagnode)
		if err != nil {
			return nil, err
		}

		err = params.addNode(dagnode, s.FileName())
		return dagnode, err
	}

	// if the progress flag was specified, wrap the file so that we can send
	// progress updates to the client (over the output channel)
	var reader io.Reader = file
	if params.progress {
		reader = &progressReader{file: file, out: params.out}
	}

	dagnode, err := add(params.node, reader, params.trickle, params.chunker)
	if err != nil {
		return nil, err
	}

	// patch it into the root
	log.Infof("adding file: %s", file.FileName())
	err = params.addNode(dagnode, file.FileName())
	return dagnode, err
}

func (params *adder) addDir(file files.File) (*dag.Node, error) {
	tree := &dag.Node{Data: ft.FolderPBData()}
	log.Infof("adding directory: %s", file.FileName())

	for {
		file, err := file.NextFile()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if file == nil {
			break
		}

		node, err := params.addFile(file)
		if _, ok := err.(*hiddenFileError); ok {
			// hidden file error, set the node to nil for below
			node = nil
		} else if err != nil {
			return nil, err
		}

		if node != nil {
			_, name := path.Split(file.FileName())

			err = tree.AddNodeLink(name, node)
			if err != nil {
				return nil, err
			}
		}
	}

	if err := params.addNode(tree, file.FileName()); err != nil {
		return nil, err
	}

	k, err := params.node.DAG.Add(tree)
	if err != nil {
		return nil, err
	}

	params.node.Pinning.GetManual().PinWithMode(k, pin.Indirect)

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

type hiddenFileError struct {
	fileName string
}

func (e *hiddenFileError) Error() string {
	return fmt.Sprintf("%s is a hidden file", e.fileName)
}

type ignoreFileError struct {
	fileName string
}

func (e *ignoreFileError) Error() string {
	return fmt.Sprintf("%s is an ignored file", e.fileName)
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

// TODO: generalize this to more than unix-fs nodes.
func newDirNode() *dag.Node {
	return &dag.Node{Data: ft.FolderPBData()}
}
