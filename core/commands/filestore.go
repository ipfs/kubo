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
	cmds "github.com/ipfs/go-ipfs/commands"
	cli "github.com/ipfs/go-ipfs/commands/cli"
	files "github.com/ipfs/go-ipfs/commands/files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/filestore"
	fsutil "github.com/ipfs/go-ipfs/filestore/util"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"gx/ipfs/QmRpAnJ1Mvd2wCtwoFevW8pbLTivUqmFxynptG6uvp1jzC/safepath"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
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
		"mv":       moveIntoFilestore,
		"enable":   FilestoreEnable,
		"disable":  FilestoreDisable,

		"verify-post-orphan": verifyPostOrphan,
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
			cwd, err = safepath.EnvWd()
		}
		if physical {
			cwd, err = safepath.SystemWd()
		}
		if err != nil {
			return err
		}
		if cwd != "" {
			paths := req.Arguments()
			for i, path := range paths {
				abspath, err := safepath.AbsPath(cwd, path)
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
		if serverSide && !config.Filestore.APIServerSidePathsEnabled() {
			res.SetError(errors.New("server side paths not enabled"), cmds.ErrNormal)
			return
		}
		if serverSide {
			err := getFiles(req)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		} else if !node.LocalMode() {
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
	f.paths = f.paths[1:]
	if f0.IsDirectory() {
		return nil, fmt.Errorf("online directory add not supported, try '-S': %s", path)
	} else if _, ok := f0.(*files.MultipartFile); !ok {
		return nil, fmt.Errorf("online adding of special files not supported, try '-S': %s", path)
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
			return 0, fmt.Errorf("%s: server side file is larger", f.FullPath())
		}
	}
	if !bytes.Equal(res, f.buf) {
		return 0, fmt.Errorf("%s: %s: server side file contents differ", f.content.FullPath(), f.local.FullPath())
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

const listingCommonText = `
If one or more <obj> is specified only list those specific objects,
otherwise list all objects.  An <obj> can either be a multihash, or an
absolute path.  If the path ends in '/' than it is assumed to be a
directory and all paths with that directory are included.
`

var lsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects in filestore.",
		ShortDescription: `
List objects in the filestore.
` + listingCommonText + `
If --all is specified list all matching blocks are lists, otherwise
only blocks representing the a file root is listed.  A file root is any
block that represents a complete file.

The default output format normally is:
  <hash> /<filepath>/<offset>
for most entries, if there are multiple files with the same content
then the first file will be as above and the others will be displayed
without the space between the <hash> and '/' to form a unique key that
can be used to reference that particular entry.  

If --format is "hash" than only the hash will be displayed.  

If --format is "key" than the full key will be displayed.

If --format is "w/type" then the type of the entry is also given
before the hash.  Type is one of:
  leaf: to indicate a node where the contents are stored
        to in the file itself
  root: to indicate a root node that represents the whole file
  other: some other kind of node that represent part of a file
  invld: a leaf node that has been found invalid

If --format is "long" then the format is:
  <type> <size> [<modtime>] <hash>[ ]/<filepath>/<offset>
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("obj", false, true, "Hash(es), filename(s), or filestore keys to list."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Alias for --format=hash."),
		cmds.BoolOption("all", "a", "List everything, not just file roots."),
		cmds.StringOption("format", "f", "Format of listing, one of: hash key default w/type long").Default("default"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		format, _, _ := req.Option("format").String()
		quiet, _, _ := req.Option("quiet").Bool()
		all, _, _ := req.Option("all").Bool()

		if quiet {
			format = "hash"
		}

		formatFun, err := fsutil.StrToFormatFun(format)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		ch, err := getListing(fs, req.Arguments(), all, format == "hash" || format == "key")

		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&chanWriter{
			ch: ch,
			format: func(r *fsutil.ListRes) (string,error) {
				return formatFun(r),nil
			},
		})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			return res.(io.Reader), nil
		},
	},
}

func procListArgs(objs []string) ([]*filestore.DbKey, fsutil.ListFilter, error) {
	keys := make([]*filestore.DbKey, 0)
	paths := make([]string, 0)
	for _, obj := range objs {
		if filepath.IsAbs(obj) {
			paths = append(paths, safepath.Clean(obj))
		} else {
			key, err := filestore.ParseKey(obj)
			if err != nil {
				return nil, nil, err
			}
			keys = append(keys, key)
		}
	}
	if len(keys) > 0 && len(paths) > 0 {
		return nil, nil, errors.New("cannot specify both hashes and paths")
	}
	if len(keys) > 0 {
		return keys, nil, nil
	} else if len(paths) > 0 {
		return nil, func(r *filestore.DataObj) bool {
			return pathMatch(paths, r.FilePath)
		}, nil
	} else {
		return nil, nil, nil
	}
}

func getListing(ds *filestore.Datastore, objs []string, all bool, keysOnly bool) (<-chan fsutil.ListRes, error) {
	keys, listFilter, err := procListArgs(objs)
	if err != nil {
		return nil, err
	}

	fs := ds.AsBasic()

	if len(keys) > 0 {
		return fsutil.ListByKey(fs, keys)
	}

	// Add filter filters if necessary
	if !all {
		if listFilter == nil {
			listFilter = fsutil.ListFilterWholeFile
		} else {
			origFilter := listFilter
			listFilter = func(r *filestore.DataObj) bool {
				return fsutil.ListFilterWholeFile(r) && origFilter(r)
			}
		}
	}

	return fsutil.List(fs, listFilter, keysOnly)
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
List files in the filestore.
` + listingCommonText + `
If --quiet is specified only the file names are printed, otherwise the
fields are as follows:
  <filepath> <hash> <size>
`,
	},
	Arguments: lsFileStore.Arguments,
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
		ch, err := getListing(fs, req.Arguments(), false, false)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
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
	format       func(*fsutil.ListRes) (string,error)
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

		line, err := w.format(&res)
		w.buf = line
		if err != nil {
			w.errs = append(w.errs, fmt.Sprintf("%s: %s", res.MHash(), err.Error()))
		}
	}
	sz := copy(p, w.buf[w.offset:])
	w.offset += sz
	return sz, nil
}

func formatDefault(res *fsutil.ListRes) (string, error) {
	return res.FormatDefault(), nil
}

func formatHash(res *fsutil.ListRes) (string, error) {
	return res.FormatHashOnly(), nil
}

func formatPorcelain(res *fsutil.ListRes) (string, error) {
	if res.Key.Hash == "" {
		return "", nil
	}
	if res.DataObj == nil {
		return fmt.Sprintf("%s\t%s\t%s\t%s\n", "block", res.StatusStr(), res.MHash(), ""), nil
	}
	pos := strings.IndexAny(res.FilePath, "\t\r\n")
	if pos == -1 {
		return fmt.Sprintf("%s\t%s\t%s\t%s\n", res.What(), res.StatusStr(), res.MHash(), res.FilePath), nil
	} else {
		str := fmt.Sprintf("%s\t%s\t%s\t%s\n", res.What(), res.StatusStr(), res.MHash(), "")
		err := errors.New("not displaying filename with tab or newline character")
		return str, err
	}
}

func formatFileName(res *fsutil.ListRes) (string, error) {
	return fmt.Sprintf("%s\n", res.FilePath), nil
}

func formatByFile(res *fsutil.ListRes) (string, error) {
	return fmt.Sprintf("%s %s %d\n", res.FilePath, res.MHash(), res.Size), nil
}

var verifyFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore.",
		ShortDescription: `
Verify <hash> nodes in the filestore.  If no hashes are specified then
verify everything in the filestore.

The normal output is:
  <status> <hash> /<filepath>//<offset>
where <hash> <type>, <filepath> are the same as in the 'ls' command
and <status> is one of:

  ok:       the original data can be reconstructed
  complete: all the blocks in the tree exists but no attempt was
            made to reconstruct the original data

  problem: some of the blocks of the tree could not be read
  incomplete: some of the blocks of the tree are missing

  changed: the contents of the backing file have changed
  no-file: the backing file could not be found
  error:   the backing file was found but could not be read

  touched: the modtimes differ but the contents where not rechecked

  ERROR:   the block could not be read due to an internal error

  found:   the child of another node was found outside the filestore
  missing: the child of another node does not exist
  <blank>: the child of another node node exists but no attempt was
           made to verify it

  appended: the node is still valid but the original file was appended

  orphan: the node is a child of another node that was not found in
          the filestore

If any checks failed than a non-zero exit status will be returned.
 
If --basic is specified linearly scan the leaf nodes to verify that they
are still valid.  Otherwise attempt to reconstruct the contents of all
nodes and check for orphan nodes if applicable.

Otherwise, the nodes are recursively visited from the root node.  If
--skip-orphans is not specified than the results are cached in memory in
order to detect the orphans.  The cache is also used to avoid visiting
the same node more than once.  Cached results are printed without any
object info.

The --level option specifies how thorough the checks should be.  The
current meaning of the levels are:
  7-9: always check the contents
    6: check the contents based on the setting of Filestore.Verify
  4-5: check the contents if the modification time differs
  2-3: report if the modification time differs
  0-1: only check for the existence of blocks without verifying the
       contents of leaf nodes

The --verbose option specifies what to output.  The current values are:
  0-1: show top-level nodes when status is not 'ok', 'complete' or '<blank>'
    2: in addition, show all nodes specified on command line
  3-4: in addition, show all top-level nodes
  5-6: in addition, show problem children
  7-9: in addition, show all children

If --porcelain is used us an alternative output is used that will not
change between releases.  The output is:
  <type0>\t<status>\t<hash>\t<filename>
where <type0> is either "root" for a file root or something else
otherwise and \t is a literal literal tab character.  <status> is the
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
		cmds.BoolOption("no-obj-info", "q", "Just print the status and the hash."),
		cmds.StringOption("incomplete-when", "Internal option."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		args := req.Arguments()
		keys, filter, err := procListArgs(args)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		basic, _, _ := req.Option("basic").Bool()
		porcelain, _, _ := req.Option("porcelain").Bool()

		params := fsutil.VerifyParams{Filter: filter}
		params.Level, _, _ = req.Option("level").Int()
		params.Verbose, _, _ = req.Option("verbose").Int()
		params.SkipOrphans, _, _ = req.Option("skip-orphans").Bool()
		params.NoObjInfo, _, _ = req.Option("no-obj-info").Bool()
		params.IncompleteWhen = getIncompleteWhenOpt(req)

		var ch <-chan fsutil.ListRes
		if basic && len(keys) == 0 {
			ch, err = fsutil.VerifyBasic(fs.AsBasic(), &params)
		} else if basic {
			ch, err = fsutil.VerifyKeys(keys, node, fs.AsBasic(), &params)
		} else if len(keys) == 0 {
			snapshot, err0 := fs.GetSnapshot()
			if err0 != nil {
				res.SetError(err0, cmds.ErrNormal)
				return
			}
			ch, err = fsutil.VerifyFull(node, snapshot, &params)
		} else {
			ch, err = fsutil.VerifyKeysFull(keys, node, fs.AsBasic(), &params)
		}
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if porcelain {
			res.SetOutput(&chanWriter{ch: ch, format: formatPorcelain, ignoreFailed: true})
		} else if params.NoObjInfo {
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

func getIncompleteWhenOpt(req cmds.Request) []string {
	str, _, _ := req.Option("incomplete-when").String()
	if str == "" {
		return nil
	} else {
		return strings.Split(str, ",")
	}
}

var verifyPostOrphan = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore and check for would be orphans.",
		ShortDescription: `
Like "verify" but perform an extra scan to check for would be orphans if
"incomplete" blocks are removed.  Becuase of how it operates only the status
and hashes are returned and the order in which blocks are reported in not
stable.

This is the method normally used by "clean".
`,
	},
	Options: []cmds.Option{
		cmds.IntOption("level", "l", "0-9, Verification level.").Default(6),
		cmds.StringOption("incomplete-when", "Internal option."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		level, _, _ := req.Option("level").Int()
		incompleteWhen := getIncompleteWhenOpt(req)

		snapshot, err := fs.GetSnapshot()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		ch, err := fsutil.VerifyPostOrphan(node, snapshot, level, incompleteWhen)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(&chanWriter{ch: ch, format: formatDefault})
		return
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

If incomplete is specified in combination with "changed", "no-file", or
"error" than any nodes that will become incomplete, after the invalid leafs
are removed, are also removed.  Similarly if "orphan" is specified in
combination with "incomplete" any would be orphans are also removed.

If the command is run with the daemon is running the check is done on a
snapshot of the filestore when it is in a consistent state.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("what", true, true, "any of: changed no-file error incomplete orphan invalid full"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Produce less output."),
		cmds.IntOption("level", "l", "0-9, Verification level.").Default(6),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		quiet, _, err := req.Option("quiet").Bool()
		level, _, _ := req.Option("level").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		//_ = node
		//ch, err := fsutil.List(fs, quiet)
		rdr, err := fsutil.Clean(req, node, fs, quiet, level, req.Arguments()...)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(rdr)
		//res.SetOutput(&chanWriter{ch, "", 0, false})
		return
	},
	//Marshalers: cmds.MarshalerMap{
	//	cmds.Text: func(res cmds.Response) (io.Reader, error) {
	//		return res.(io.Reader), nil
	//	},
	//},
}

var rmFilestoreObjs = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove entries from the filestore.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, true, "Objects to remove."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("force", "f", "Ignore nonexistent blocks.").Default(false),
		cmds.BoolOption("quiet", "q", "Write minimal output.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, fs, err := extractFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		hashes := req.Arguments()
		//force, _, _ := req.Option("force").Bool()
		//quiet, _, _ := req.Option("quiet").Bool()
		keys := make([]*filestore.DbKey, 0, len(hashes))
		for _, hash := range hashes {
			k, err := filestore.ParseKey(hash)
			if err != nil {
				res.SetError(fmt.Errorf("invalid filestore key: %s (%s)", hash, err), cmds.ErrNormal)
				return
			}
			keys = append(keys, k)
		}
		outChan := make(chan interface{})
		err = fsutil.RmBlocks(fs, n.Blockstore, n.Pinning, outChan, keys)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput((<-chan interface{})(outChan))
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
		err := errors.New("filestore not enabled")
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
			err := fsutil.Dups(w, fs.AsBasic(), node.Blockstore, node.Pinning, req.Arguments()...)
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
		local := node.LocalMode()
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
		k, err := cid.Decode(mhash)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		path := ""
		if len(args) == 2 {
			path = args[1]
		} else {
			path = mhash
		}
		if local {
			path, err = filepath.Abs(path)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}
		rdr, wtr := io.Pipe()
		go func() {
			err := fsutil.ConvertToFile(node, k, path)
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

var FilestoreEnable = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Enable the filestore.",
		ShortDescription: `
Enable the filestore.  A noop if the filestore is already enabled.
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		rootDir := req.InvocContext().ConfigRoot
		err := fsrepo.InitFilestore(rootDir)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var FilestoreDisable = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Disable an empty filestore.",
		ShortDescription: `
Disable the filestore if it is empty.  A noop if the filestore does
not exist.  An error if the filestore is not empty.
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		res.SetError(errors.New("unimplemented"), cmds.ErrNormal)
	},
}
