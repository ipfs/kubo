package commands

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/cheggaaa/pb"
	ignore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/sabhiram/go-git-ignore"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	cmds "github.com/ipfs/go-ipfs/commands"
	files "github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	importer "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
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
	hiddenOptionName   = "hidden"
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
		cmds.BoolOption(hiddenOptionName, "Include files that are hidden"),
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
		hidden, _, _ := req.Option(hiddenOptionName).Bool()

		var ignoreFilePatterns []ignore.GitIgnore

		// Check the IPFS_PATH
		if ipfs_path := req.Context().ConfigRoot; len(ipfs_path) > 0 {
			baseFilePattern, err := ignore.CompileIgnoreFile(path.Join(ipfs_path, ".ipfsignore"))
			if err == nil && baseFilePattern != nil {
				ignoreFilePatterns = append(ignoreFilePatterns, *baseFilePattern)
			}
		}

		outChan := make(chan interface{})
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

				// If the file is not a folder, then let's get the root of that
				// folder and attempt to load the appropriate .ipfsignore.
				localIgnorePatterns := checkForParentIgnorePatterns(file.FileName(), ignoreFilePatterns)

				addParams := adder{n, outChan, progress, wrap, hidden}
				rootnd, err := addParams.addFile(file, localIgnorePatterns)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				err = n.Pinning.Pin(context.Background(), rootnd, true)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

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

// Internal structure for holding the switches passed to the `add` call
type adder struct {
	node     *core.IpfsNode
	out      chan interface{}
	progress bool
	wrap     bool
	hidden   bool
}

// Perform the actual add & pin locally, outputting results to reader
func add(n *core.IpfsNode, reader io.Reader) (*dag.Node, error) {
	node, err := importer.BuildDagFromReader(reader, n.DAG, nil, chunk.DefaultSplitter)
	if err != nil {
		return nil, err
	}

	err = n.Pinning.Flush()
	if err != nil {
		return nil, err
	}

	return node, nil
}

// Add the given file while respecting the params and ignoreFilePatterns.
// Note that ignoreFilePatterns is not part of the struct as it may change while
// we dig through folders.
func (params *adder) addFile(file files.File, ignoreFilePatterns []ignore.GitIgnore) (*dag.Node, error) {
	// Check if file is hidden
	if fileIsHidden := files.IsHidden(file); fileIsHidden && !params.hidden {
		log.Debugf("%s is hidden, skipping", file.FileName())
		return nil, &hiddenFileError{file.FileName()}
	}

	// Check for ignore files matches
	for i := range ignoreFilePatterns {
		if ignoreFilePatterns[i].MatchesPath(file.FileName()) {
			log.Debugf("%s is ignored file, skipping", file.FileName())
			return nil, &ignoreFileError{file.FileName()}
		}
	}

	// Check if "file" is actually a directory
	if file.IsDirectory() {
		return params.addDir(file, ignoreFilePatterns)
	}

	// if the progress flag was specified, wrap the file so that we can send
	// progress updates to the client (over the output channel)
	var reader io.Reader = file
	if params.progress {
		reader = &progressReader{file: file, out: params.out}
	}

	if params.wrap {
		p, dagnode, err := coreunix.AddWrapped(params.node, reader, path.Base(file.FileName()))
		if err != nil {
			return nil, err
		}
		params.out <- &AddedObject{
			Hash: p,
			Name: file.FileName(),
		}
		return dagnode, nil
	}

	dagnode, err := add(params.node, reader)
	if err != nil {
		return nil, err
	}

	log.Infof("adding file: %s", file.FileName())
	if err := outputDagnode(params.out, file.FileName(), dagnode); err != nil {
		return nil, err
	}
	return dagnode, nil
}

func (params *adder) addDir(file files.File, ignoreFilePatterns []ignore.GitIgnore) (*dag.Node, error) {

	tree := &dag.Node{Data: ft.FolderPBData()}
	log.Infof("adding directory: %s", file.FileName())

	// Check for an .ipfsignore file that is local to this Dir and append to the incoming
	localIgnorePatterns := checkForLocalIgnorePatterns(file.FileName(), ignoreFilePatterns)

	for {
		file, err := file.NextFile()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if file == nil {
			break
		}

		node, err := params.addFile(file, localIgnorePatterns)
		if _, ok := err.(*hiddenFileError); ok {
			// hidden file error, set the node to nil for below
			node = nil
		} else if _, ok := err.(*ignoreFileError); ok {
			// ignore file error, set the node to nil for below
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

	err := outputDagnode(params.out, file.FileName(), tree)
	if err != nil {
		return nil, err
	}

	_, err = params.node.DAG.Add(tree)
	if err != nil {
		return nil, err
	}

	return tree, nil
}

// this helper checks the local path for any .ipfsignore file that need to be
// respected. returns the updated or the original GitIgnore.
func checkForLocalIgnorePatterns(dir string, ignoreFilePatterns []ignore.GitIgnore) []ignore.GitIgnore {

	ignorePathname := path.Join(dir, ".ipfsignore")

	localIgnore, ignoreErr := ignore.CompileIgnoreFile(ignorePathname)
	if ignoreErr == nil && localIgnore != nil {
		log.Debugf("found ignore file: %s", dir)
		return append(ignoreFilePatterns, *localIgnore)
	} else {
		return ignoreFilePatterns
	}
}

// this helper just walks the parent directories of the given path looking for
// any .ipfsignore files in those directories.
func checkForParentIgnorePatterns(givenPath string, ignoreFilePatterns []ignore.GitIgnore) []ignore.GitIgnore {
	absolutePath, err := filepath.Abs(givenPath)

	if err != nil {
		return ignoreFilePatterns
	}

	// break out the absolute path
	dir := filepath.Dir(absolutePath)
	pathComponents := strings.Split(dir, string(filepath.Separator))

	// We loop through each parent component attempting to find an .ipfsignore file
	for index, _ := range pathComponents {

		pathParts := make([]string, len(pathComponents)+1)
		copy(pathParts, pathComponents[0:index+1])
		ignorePathname := path.Join(append(pathParts, ".ipfsignore")...)

		localIgnore, ignoreErr := ignore.CompileIgnoreFile(ignorePathname)
		if ignoreErr == nil && localIgnore != nil {
			log.Debugf("found ignore file: %s", ignorePathname)
			ignoreFilePatterns = append(ignoreFilePatterns, *localIgnore)
		}
	}

	return ignoreFilePatterns
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
