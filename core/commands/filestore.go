package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	//"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	//ds "github.com/ipfs/go-datastore"
	//bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	cli "github.com/ipfs/go-ipfs/commands/cli"
	files "github.com/ipfs/go-ipfs/commands/files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/filestore"
	fsutil "github.com/ipfs/go-ipfs/filestore/util"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filestore objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"add":      addFileStore,
		"ls":       lsFileStore,
		"ls-files": lsFiles,
		"verify":   verifyFileStore,
		"rm":       rmFilestoreObjs,
		"clean":    cleanFileStore,
		"dups":     fsDups,
		"upgrade":  fsUpgrade,
		"mv":       moveIntoFilestore,
	},
}

var addFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add files to the filestore.",
		ShortDescription: `
Add contents of <path> to the filestore.  Most of the options are the
same as for 'ipfs add'.
`},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, true, "The path to a file to be added."),
	},
	Options: addFileStoreOpts(),
	PreRun: func(req cmds.Request) error {
		serverSide, _, _ := req.Option("server-side").Bool()
		logical, _, _ := req.Option("logical").Bool()
		physical, _, _ := req.Option("physical").Bool()
		if logical && physical {
			return errors.New("both --logical and --physical can not be specified")
		}
		cwd := ""
		var err error
		if logical {
			cwd, err = filestore.EnvWd()
		}
		if physical {
			cwd, err = filestore.SystemWd()
		}
		if err != nil {
			return err
		}
		if cwd != "" {
			paths := req.Arguments()
			for i, path := range paths {
				abspath, err := filestore.AbsPath(cwd, path)
				if err != nil {
					return err
				}
				paths[i] = abspath
			}
			req.SetArguments(paths)
		}
		if !serverSide {
			err := getFiles(req)
			if err != nil {
				return err
			}
		}
		return AddCmd.PreRun(req)
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		config, _ := req.InvocContext().GetConfig()
		serverSide, _, _ := req.Option("server-side").Bool()
		if serverSide && !config.Filestore.APIServerSidePaths {
			res.SetError(errors.New("server side paths not enabled"), cmds.ErrNormal)
			return
		}
		if serverSide {
			err := getFiles(req)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		} else if node.OnlineMode() {
			if !req.Files().IsDirectory() {
				res.SetError(errors.New("expected directory object"), cmds.ErrNormal)
				return
			}
			req.SetFiles(&fixPath{req.Arguments(), req.Files()})
		}
		req.Values()["no-copy"] = true
		AddCmd.Run(req, res)
	},
	PostRun: AddCmd.PostRun,
	Type:    AddCmd.Type,
}

func addFileStoreOpts() []cmds.Option {
	var opts []cmds.Option

	foundPinOpt := false
	for _, opt := range AddCmd.Options {
		if opt.Names()[0] == pinOptionName {
			opts = append(opts, cmds.BoolOption(pinOptionName, opt.Description()).Default(false))
			foundPinOpt = true
		} else {
			opts = append(opts, opt)
		}
	}
	if !foundPinOpt {
		panic("internal error: foundPinOpt is false")
	}

	opts = append(opts,
		cmds.BoolOption("server-side", "S", "Read file on server."),
		cmds.BoolOption("logical", "l", "Create absolute path using PWD from environment."),
		cmds.BoolOption("physical", "P", "Create absolute path using a system call."),
	)
	return opts
}

func getFiles(req cmds.Request) error {
	inputs := req.Arguments()
	for _, fn := range inputs {
		if !filepath.IsAbs(fn) {
			return fmt.Errorf("file path must be absolute: %s", fn)
		}
	}
	_, fileArgs, err := cli.ParseArgs(req, inputs, nil, AddCmd.Arguments, nil)
	if err != nil {
		return err
	}
	file := files.NewSliceFile("", "", fileArgs)
	req.SetFiles(file)
	names := make([]string, len(fileArgs))
	for i, f := range fileArgs {
		names[i] = f.FullPath()
	}
	req.SetArguments(names)
	return nil
}

type fixPath struct {
	paths []string
	orig  files.File
}

func (f *fixPath) IsDirectory() bool            { return true }
func (f *fixPath) Read(res []byte) (int, error) { return 0, io.EOF }
func (f *fixPath) FileName() string             { return f.orig.FileName() }
func (f *fixPath) FullPath() string             { return f.orig.FullPath() }
func (f *fixPath) Close() error                 { return f.orig.Close() }

func (f *fixPath) NextFile() (files.File, error) {
	f0, _ := f.orig.NextFile()
	if f0 == nil {
		return nil, io.EOF
	}
	if len(f.paths) == 0 {
		return nil, errors.New("len(req.Files()) < len(req.Arguments())")
	}
	path := f.paths[0]
	f.paths = f.paths[:1]
	if f0.IsDirectory() {
		return nil, errors.New("online directory add not supported, try '-S'")
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		stat, err := f.Stat()
		if err != nil {
			return nil, err
		}
		return &dualFile{
			content: f0,
			local:   files.NewReaderFile(f0.FileName(), path, f, stat),
		}, nil
	}
}

type dualFile struct {
	content files.File
	local   files.StatFile
	buf     []byte
}

func (f *dualFile) IsDirectory() bool             { return false }
func (f *dualFile) NextFile() (files.File, error) { return nil, files.ErrNotDirectory }
func (f *dualFile) FileName() string              { return f.local.FileName() }
func (f *dualFile) FullPath() string              { return f.local.FullPath() }
func (f *dualFile) Stat() os.FileInfo             { return f.local.Stat() }
func (f *dualFile) Size() (int64, error)          { return f.local.Stat().Size(), nil }

func (f *dualFile) Read(res []byte) (int, error) {
	// First read the content send from the client
	n, err1 := f.content.Read(res)
	if err1 == io.ErrUnexpectedEOF { // avoid this special case
		err1 = io.EOF
	}
	if err1 != nil && err1 != io.EOF {
		return 0, err1
	}
	res = res[:n]

	// Next try to read the same amount of data from the local file
	if n == 0 && err1 == io.EOF {
		// Make sure we try to read at least one byte in order
		// to get an EOF
		n = 1
	}
	if cap(f.buf) < n {
		f.buf = make([]byte, n)
	} else {
		f.buf = f.buf[:n]
	}
	n, err := io.ReadFull(f.local, f.buf)
	if err == io.ErrUnexpectedEOF { // avoid this special case
		err = io.EOF
	}
	if err != nil && err != io.EOF {
		return 0, err
	}
	f.buf = f.buf[:n]

	// Now compare the results and return an error if the contents
	// sent from the client differ from the contents of the file
	if len(res) == 0 && err1 == io.EOF {
		if len(f.buf) == 0 && err == io.EOF {
			return 0, io.EOF
		} else {
			return 0, errors.New("server side file is larger")
		}
	}
	if !bytes.Equal(res, f.buf) {
		return 0, errors.New("file contents differ")
	}
	return n, err1
}

func (f *dualFile) Close() error {
	err := f.content.Close()
	if err != nil {
		return err
	}
	return f.local.Close()
}

var lsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects in filestore.",
		ShortDescription: `
List objects in the filestore.  If one or more <obj> is specified only
list those specific objects, otherwise list all objects.  An <obj> can
either be a multihash, or an absolute path.  If the path ends in '/'
than it is assumed to be a directory and all paths with that directory
are included.

If --all is specified list all matching blocks are lists, otherwise
only blocks representing the a file root is listed.  A file root is any
block that represents a complete file.

If --quiet is specified only the hashes are printed, otherwise the
fields are as follows:
  <hash> <type> <filepath> <offset> <size> [<modtime>]
where <type> is one of:"
  leaf: to indicate a node where the contents are stored
        to in the file itself
  root: to indicate a root node that represents the whole file
  other: some other kind of node that represent part of a file
  invld: a leaf node that has been found invalid
and <filepath> is the part of the file the object represents.  The
part represented starts at <offset> and continues for <size> bytes.
If <offset> is the special value "-" indicates a file root.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("obj", false, true, "Hash or filename to list."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write just hashes of objects."),
		cmds.BoolOption("all", "a", "List everything, not just file roots."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := req.Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		all, _, err := req.Option("all").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		objs := req.Arguments()
		keys := make([]k.Key, 0)
		paths := make([]string, 0)
		for _, obj := range objs {
			if filepath.IsAbs(obj) {
				paths = append(paths, filestore.CleanPath(obj))
			} else {
				keys = append(keys, k.B58KeyDecode(obj))
			}
		}
		if len(keys) > 0 && len(paths) > 0 {
			res.SetError(errors.New("cannot specify both hashes and paths"), cmds.ErrNormal)
			return
		}

		var ch <-chan fsutil.ListRes
		if len(keys) > 0 {
			ch, _ = fsutil.ListByKey(fs, keys)
		} else if all && len(paths) == 0 && quiet {
			ch, _ = fsutil.ListKeys(fs)
		} else if all && len(paths) == 0 {
			ch, _ = fsutil.ListAll(fs)
		} else if !all && len(paths) == 0 {
			ch, _ = fsutil.ListWholeFile(fs)
		} else if all {
			ch, _ = fsutil.List(fs, func(r fsutil.ListRes) bool {
				return pathMatch(paths, r.FilePath)
			})
		} else {
			ch, _ = fsutil.List(fs, func(r fsutil.ListRes) bool {
				return r.WholeFile() && pathMatch(paths, r.FilePath)
			})
		}

		if quiet {
			res.SetOutput(&chanWriter{ch: ch, format: formatHash})
		} else {
			res.SetOutput(&chanWriter{ch: ch, format: formatDefault})
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

func pathMatch(match_list []string, path string) bool {
	for _, to_match := range match_list {
		if to_match[len(to_match)-1] == filepath.Separator {
			if strings.HasPrefix(path, to_match) {
				return true
			}
		} else {
			if to_match == path {
				return true
			}
		}
	}
	return false

}

var lsFiles = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List files in filestore.",
		ShortDescription: `
List files in the filestore.  If --quiet is specified only the
file names are printed, otherwise the fields are as follows:
  <filepath> <hash> <size>
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write just filenames."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := req.Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		ch, _ := fsutil.ListWholeFile(fs)
		if quiet {
			res.SetOutput(&chanWriter{ch: ch, format: formatFileName})
		} else {
			res.SetOutput(&chanWriter{ch: ch, format: formatByFile})
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

type chanWriter struct {
	ch           <-chan fsutil.ListRes
	buf          string
	offset       int
	checksFailed bool
	ignoreFailed bool
	errs         []string
	format       func(fsutil.ListRes) (string, error)
}

func (w *chanWriter) Read(p []byte) (int, error) {
	if w.offset >= len(w.buf) {
		w.offset = 0
		res, more := <-w.ch

		if !more {
			if w.checksFailed {
				w.errs = append(w.errs, "some checks failed")
			}
			if len(w.errs) == 0 {
				return 0, io.EOF
			} else {
				return 0, errors.New(strings.Join(w.errs, ".  "))
			}
		}

		if !w.ignoreFailed && fsutil.AnError(res.Status) {
			w.checksFailed = true
		}

		line, err := w.format(res)
		w.buf = line
		if err != nil {
			w.errs = append(w.errs, fmt.Sprintf("%s: %s", res.MHash(), err.Error()))
		}
	}
	sz := copy(p, w.buf[w.offset:])
	w.offset += sz
	return sz, nil
}

func formatDefault(res fsutil.ListRes) (string, error) {
	return res.Format(), nil
}

func formatHash(res fsutil.ListRes) (string, error) {
	return fmt.Sprintf("%s\n", res.MHash()), nil
}

func formatPorcelain(res fsutil.ListRes) (string, error) {
	if len(res.RawHash()) == 0 {
		return "", nil
	}
	if res.DataObj == nil {
		return "", fmt.Errorf("key not found: %s", res.MHash())
	}
	pos := strings.IndexAny(res.FilePath, "\t\r\n")
	if pos == -1 {
		return fmt.Sprintf("%s\t%s\t%s\t%s\n", res.What(), res.StatusStr(), res.MHash(), res.FilePath), nil
	} else {
		str := fmt.Sprintf("%s\t%s\t%s\t%s\n", res.What(), res.StatusStr(), res.MHash(), "ERROR")
		err := errors.New("not displaying filename with tab or newline character")
		return str, err
	}
}

func formatFileName(res fsutil.ListRes) (string, error) {
	return fmt.Sprintf("%s\n", res.FilePath), nil
}

func formatByFile(res fsutil.ListRes) (string, error) {
	return fmt.Sprintf("%s %s %d\n", res.FilePath, res.MHash(), res.Size), nil
}

var verifyFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore.",
		ShortDescription: `
Verify <hash> nodes in the filestore.  If no hashes are specified then
verify everything in the filestore.

The normal output is:
  <status> <hash> [<type> <filepath> <offset> <size> [<modtime>]]
where <hash> <type>, <filepath>, <offset>, <size> and <modtime>
are the same as in the 'ls' command and <status> is one of:

  ok:       the original data can be reconstructed
  complete: all the blocks in the tree exists but no attempt was
            made to reconstruct the original data

  incomplete: some of the blocks of the tree could not be read

  changed: the contents of the backing file have changed
  no-file: the backing file could not be found
  error:   the backing file was found but could not be read

  ERROR:   the block could not be read due to an internal error

  found:   the child of another node was found outside the filestore
  missing: the child of another node does not exist
  <blank>: the child of another node node exists but no attempt was
           made to verify it

  appended: the node is still valid but the original file was appended

  orphan: the node is a child of another node that was not found in
          the filestore

If any checks failed than a non-zero exit status will be returned.
 
If --basic is specified then just scan leaf nodes to verify that they
are still valid.  Otherwise attempt to reconstruct the contents of
all nodes and check for orphan nodes if applicable.

The --level option specifies how thorough the checks should be.  The
current meaning of the levels are:
  7-9: always check the contents
  2-6: check the contents if the modification time differs
  0-1: only check for the existence of blocks without verifying the
       contents of leaf nodes

The --verbose option specifies what to output.  The current values are:
  7-9: show everything
  5-6: don't show child nodes unless there is a problem
  3-4: don't show child nodes
    2: don't show uninteresting root nodes
  0-1: don't show uninteresting specified hashes
uninteresting means a status of 'ok' or '<blank>'

If --porcelain is used us an alternative output is used that will not
change between releases.  The output is:
  <type0>\\t<status>\\t<hash>\\t<filename>
where <type0> is either "root" for a file root or something else
otherwise and \\t is a literal literal tab character.  <status> is the
same as normal except that <blank> is spelled out as "unchecked".  In
addition to the modified output a non-zero exit status will only be
returned on an error condition and not just because of failed checks.
In the event that <filename> contains a tab or newline character the
filename will not be displayed (and a non-zero exit status will be
returned) to avoid special cases when parsing the output.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", false, true, "Hashs of nodes to verify."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("basic", "Perform a basic scan of leaf nodes only."),
		cmds.IntOption("level", "l", "0-9, Verification level.").Default(6),
		cmds.IntOption("verbose", "v", "0-9 Verbose level.").Default(6),
		cmds.BoolOption("porcelain", "Porcelain output."),
		cmds.BoolOption("skip-orphans", "Skip check for orphans."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		args := req.Arguments()
		keys := make([]k.Key, 0)
		for _, key := range args {
			keys = append(keys, k.B58KeyDecode(key))
		}
		basic, _, err := req.Option("basic").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		level, _, err := req.Option("level").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		verbose, _, err := req.Option("verbose").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		porcelain, _, err := req.Option("porcelain").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if level < 0 || level > 9 {
			res.SetError(errors.New("level must be between 0-9"), cmds.ErrNormal)
			return
		}
		skipOrphans, _, err := req.Option("skip-orphans").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var ch <-chan fsutil.ListRes
		if basic && len(keys) == 0 {
			ch, _ = fsutil.VerifyBasic(fs, level, verbose)
		} else if basic {
			ch, _ = fsutil.VerifyKeys(keys, node, fs, level, verbose)
		} else if len(keys) == 0 {
			ch, _ = fsutil.VerifyFull(node, fs, level, verbose, skipOrphans)
		} else {
			ch, _ = fsutil.VerifyKeysFull(keys, node, fs, level, verbose)
		}
		if porcelain {
			res.SetOutput(&chanWriter{ch: ch, format: formatPorcelain, ignoreFailed: true})
		} else {
			res.SetOutput(&chanWriter{ch: ch, format: formatDefault})
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var cleanFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove invalid or orphan nodes from the filestore.",
		ShortDescription: `
Removes invalid or orphan nodes from the filestore as specified by
<what>.  <what> is the status of a node as reported by "verify", it
can be any of "changed", "no-file", "error", "incomplete",
"orphan", "invalid" or "full".  "invalid" is an alias for "changed"
and "no-file" and "full" is an alias for "invalid" "incomplete" and
"orphan" (basically remove everything but "error").

It does the removal in three passes.  If there is nothing specified to
be removed in a pass that pass is skipped.  The first pass does a
"verify --basic" and is used to remove any "changed", "no-file" or
"error" nodes.  The second pass does a "verify --level 0
--skip-orphans" and will is used to remove any "incomplete" nodes due
to missing children (the "--level 0" only checks for the existence of
leaf nodes, but does not try to read the content).  The final pass
will do a "verify --level 0" and is used to remove any "orphan" nodes.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("what", true, true, "any of: changed no-file error incomplete orphan invalid full"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Produce less output."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := req.Option("quiet").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		//_ = node
		//ch, err := fsutil.List(fs, quiet)
		rdr, err := fsutil.Clean(req, node, fs, quiet, req.Arguments()...)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(rdr)
		//res.SetOutput(&chanWriter{ch, "", 0, false})
		return
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var rmFilestoreObjs = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove blocks from the filestore.",
	},
	Arguments: blockRmCmd.Arguments,
	Options:   blockRmCmd.Options,
	Run: func(req cmds.Request, res cmds.Response) {
		blockRmRun(req, res, fsrepo.FilestoreMount)
	},
	PostRun: blockRmCmd.PostRun,
	Type:    blockRmCmd.Type,
}

func extractFilestore(req cmds.Request) (*core.IpfsNode, *filestore.Datastore, error) {
	node, err := req.InvocContext().GetNode()
	if err != nil {
		return nil, nil, err
	}
	fs, ok := node.Repo.DirectMount(fsrepo.FilestoreMount).(*filestore.Datastore)
	if !ok {
		err := errors.New("could not extract filestore")
		return nil, nil, err
	}
	return node, fs, nil
}

var fsDups = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List duplicate blocks stored outside filestore.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("what", false, true, "any of: pinned unpinned"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			return
		}
		r, w := io.Pipe()
		go func() {
			err := fsutil.Dups(w, fs, node.Blockstore, node.Pinning, req.Arguments()...)
			if err != nil {
				w.CloseWithError(err)
			} else {
				w.Close()
			}
		}()
		res.SetOutput(r)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var fsUpgrade = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Upgrade filestore to most recent format.",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			return
		}
		r, w := io.Pipe()
		go func() {
			err := fsutil.Upgrade(w, fs)
			if err != nil {
				w.CloseWithError(err)
			} else {
				w.Close()
			}
		}()
		res.SetOutput(r)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

var moveIntoFilestore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Move a Node representing file into the filestore.",
		ShortDescription: `
Move a node representing a file into the filestore.  For now the old
copy is not removed.  Use "filestore rm-dups" to remove the old copy.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, false, "Multi-hash to move."),
		cmds.StringArg("file", false, false, "File to store node's content in."),
	},
	Options: []cmds.Option{},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		offline := !node.OnlineMode()
		args := req.Arguments()
		if len(args) < 1 {
			res.SetError(errors.New("must specify hash"), cmds.ErrNormal)
			return
		}
		if len(args) > 2 {
			res.SetError(errors.New("too many arguments"), cmds.ErrNormal)
			return
		}
		mhash := args[0]
		key := k.B58KeyDecode(mhash)
		path := ""
		if len(args) == 2 {
			path = args[1]
		} else {
			path = mhash
		}
		if offline {
			path, err = filepath.Abs(path)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}
		rdr, wtr := io.Pipe()
		go func() {
			err := fsutil.ConvertToFile(node, key, path)
			if err != nil {
				wtr.CloseWithError(err)
				return
			}
			wtr.Close()
		}()
		res.SetOutput(rdr)
		return
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}
