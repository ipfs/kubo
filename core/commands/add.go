package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/commands/cmdenv"

	"github.com/cheggaaa/pb"
	"github.com/ipfs/boxo/files"
	mfs "github.com/ipfs/boxo/mfs"
	"github.com/ipfs/boxo/path"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipld "github.com/ipfs/go-ipld-format"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	mh "github.com/multiformats/go-multihash"
)

// ErrDepthLimitExceeded indicates that the max depth has been exceeded.
var ErrDepthLimitExceeded = errors.New("depth limit exceeded")

type TimeParts struct {
	t *time.Time
}

func (t TimeParts) MarshalJSON() ([]byte, error) {
	return t.t.MarshalJSON()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// The time is expected to be a quoted string in RFC 3339 format.
func (t *TimeParts) UnmarshalJSON(data []byte) (err error) {
	// Fractional seconds are handled implicitly by Parse.
	tt, err := time.Parse("\"2006-01-02T15:04:05Z\"", string(data))
	*t = TimeParts{&tt}
	return
}

type AddEvent struct {
	Name       string
	Hash       string `json:",omitempty"`
	Bytes      int64  `json:",omitempty"`
	Size       string `json:",omitempty"`
	Mode       string `json:",omitempty"`
	Mtime      int64  `json:",omitempty"`
	MtimeNsecs int    `json:",omitempty"`
}

const (
	quietOptionName       = "quiet"
	quieterOptionName     = "quieter"
	silentOptionName      = "silent"
	progressOptionName    = "progress"
	trickleOptionName     = "trickle"
	wrapOptionName        = "wrap-with-directory"
	onlyHashOptionName    = "only-hash"
	chunkerOptionName     = "chunker"
	pinOptionName         = "pin"
	rawLeavesOptionName   = "raw-leaves"
	noCopyOptionName      = "nocopy"
	fstoreCacheOptionName = "fscache"
	cidVersionOptionName  = "cid-version"
	hashOptionName        = "hash"
	inlineOptionName      = "inline"
	inlineLimitOptionName = "inline-limit"
	toFilesOptionName     = "to-files"

	preserveModeOptionName  = "preserve-mode"
	preserveMtimeOptionName = "preserve-mtime"
	modeOptionName          = "mode"
	mtimeOptionName         = "mtime"
	mtimeNsecsOptionName    = "mtime-nsecs"
)

const adderOutChanSize = 8

var AddCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a file or directory to IPFS.",
		ShortDescription: `
Adds the content of <path> to IPFS. Use -r to add directories (recursively).
`,
		LongDescription: `
Adds the content of <path> to IPFS. Use -r to add directories.
Note that directories are added recursively, to form the IPFS
MerkleDAG.

If the daemon is not running, it will just add locally.
If the daemon is started later, it will be advertised after a few
seconds when the reprovider runs.

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

Files imported with 'ipfs add' are protected from GC (implicit '--pin=true'),
but it is up to you to remember the returned CID to get the data back later.

Passing '--to-files' creates a reference in Files API (MFS), making it easier
to find it in the future:

  > ipfs files mkdir -p /myfs/dir
  > ipfs add example.jpg --to-files /myfs/dir/
  > ipfs files ls /myfs/dir/
  example.jpg

See 'ipfs files --help' to learn more about using MFS
for keeping track of added files and directories.

The chunker option, '-s', specifies the chunking strategy that dictates
how to break files into blocks. Blocks with same content can
be deduplicated. Different chunking strategies will produce different
hashes for the same file. The default is a fixed block size of
256 * 1024 bytes, 'size-262144'. Alternatively, you can use the
Buzhash or Rabin fingerprint chunker for content defined chunking by
specifying buzhash or rabin-[min]-[avg]-[max] (where min/avg/max refer
to the desired chunk sizes in bytes), e.g. 'rabin-262144-524288-1048576'.

The following examples use very small byte sizes to demonstrate the
properties of the different chunkers on a small file. You'll likely
want to use a 1024 times larger chunk sizes for most files.

  > ipfs add --chunker=size-2048 ipfs-logo.svg
  added QmafrLBfzRLV4XSH1XcaMMeaXEUhDJjmtDfsYU95TrWG87 ipfs-logo.svg
  > ipfs add --chunker=rabin-512-1024-2048 ipfs-logo.svg
  added Qmf1hDN65tR55Ubh2RN1FPxr69xq3giVBz1KApsresY8Gn ipfs-logo.svg

You can now check what blocks have been created by:

  > ipfs object links QmafrLBfzRLV4XSH1XcaMMeaXEUhDJjmtDfsYU95TrWG87
  QmY6yj1GsermExDXoosVE3aSPxdMNYr6aKuw3nA8LoWPRS 2059
  Qmf7ZQeSxq2fJVJbCmgTrLLVN9tDR9Wy5k75DxQKuz5Gyt 1195
  > ipfs object links Qmf1hDN65tR55Ubh2RN1FPxr69xq3giVBz1KApsresY8Gn
  QmY6yj1GsermExDXoosVE3aSPxdMNYr6aKuw3nA8LoWPRS 2059
  QmerURi9k4XzKCaaPbsK6BL5pMEjF7PGphjDvkkjDtsVf3 868
  QmQB28iwSriSUSMqG2nXDTLtdPHgWb4rebBrU7Q1j4vxPv 338

Finally, a note on hash (CID) determinism and 'ipfs add' command.

Almost all the flags provided by this command will change the final CID, and
new flags may be added in the future. It is not guaranteed for the implicit
defaults of 'ipfs add' to remain the same in future Kubo releases, or for other
IPFS software to use the same import parameters as Kubo.

If you need to back up or transport content-addressed data using a non-IPFS
medium, CID can be preserved with CAR files.
See 'dag export' and 'dag import' for more information.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("path", true, true, "The path to a file to be added to IPFS.").EnableRecursive().EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.OptionRecursivePath, // a builtin option that allows recursive paths (-r, --recursive)
		cmds.OptionDerefArgs,     // a builtin option that resolves passed in filesystem links (--dereference-args)
		cmds.OptionStdinName,     // a builtin option that optionally allows wrapping stdin into a named file
		cmds.OptionHidden,
		cmds.OptionIgnore,
		cmds.OptionIgnoreRules,
		cmds.BoolOption(quietOptionName, "q", "Write minimal output."),
		cmds.BoolOption(quieterOptionName, "Q", "Write only final hash."),
		cmds.BoolOption(silentOptionName, "Write no output."),
		cmds.BoolOption(progressOptionName, "p", "Stream progress data."),
		cmds.BoolOption(trickleOptionName, "t", "Use trickle-dag format for dag generation."),
		cmds.BoolOption(onlyHashOptionName, "n", "Only chunk and hash - do not write to disk."),
		cmds.BoolOption(wrapOptionName, "w", "Wrap files with a directory object."),
		cmds.StringOption(chunkerOptionName, "s", "Chunking algorithm, size-[bytes], rabin-[min]-[avg]-[max] or buzhash"),
		cmds.BoolOption(rawLeavesOptionName, "Use raw blocks for leaf nodes."),
		cmds.BoolOption(noCopyOptionName, "Add the file using filestore. Implies raw-leaves. (experimental)"),
		cmds.BoolOption(fstoreCacheOptionName, "Check the filestore for pre-existing blocks. (experimental)"),
		cmds.IntOption(cidVersionOptionName, "CID version. Defaults to 0 unless an option that depends on CIDv1 is passed. Passing version 1 will cause the raw-leaves option to default to true."),
		cmds.StringOption(hashOptionName, "Hash function to use. Implies CIDv1 if not sha2-256. (experimental)"),
		cmds.BoolOption(inlineOptionName, "Inline small blocks into CIDs. (experimental)"),
		cmds.IntOption(inlineLimitOptionName, "Maximum block size to inline. (experimental)").WithDefault(32),
		cmds.BoolOption(pinOptionName, "Pin locally to protect added files from garbage collection.").WithDefault(true),
		cmds.StringOption(toFilesOptionName, "Add reference to Files API (MFS) at the provided path."),
		cmds.BoolOption(preserveModeOptionName, "Apply existing POSIX permissions to created UnixFS entries. Disables raw-leaves. (experimental)"),
		cmds.BoolOption(preserveMtimeOptionName, "Apply existing POSIX modification time to created UnixFS entries. Disables raw-leaves. (experimental)"),
		cmds.UintOption(modeOptionName, "Custom POSIX file mode to store in created UnixFS entries. Disables raw-leaves. (experimental)"),
		cmds.Int64Option(mtimeOptionName, "Custom POSIX modification time to store in created UnixFS entries (seconds before or after the Unix Epoch). Disables raw-leaves. (experimental)"),
		cmds.UintOption(mtimeNsecsOptionName, "Custom POSIX modification time (optional time fraction in nanoseconds)"),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		quiet, _ := req.Options[quietOptionName].(bool)
		quieter, _ := req.Options[quieterOptionName].(bool)
		quiet = quiet || quieter
		silent, _ := req.Options[silentOptionName].(bool)

		if !quiet && !silent {
			// ipfs cli progress bar defaults to true unless quiet or silent is used
			_, found := req.Options[progressOptionName].(bool)
			if !found {
				req.Options[progressOptionName] = true
			}
		}

		return nil
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		cfg, err := nd.Repo.Config()
		if err != nil {
			return err
		}

		progress, _ := req.Options[progressOptionName].(bool)
		trickle, _ := req.Options[trickleOptionName].(bool)
		wrap, _ := req.Options[wrapOptionName].(bool)
		onlyHash, _ := req.Options[onlyHashOptionName].(bool)
		silent, _ := req.Options[silentOptionName].(bool)
		chunker, _ := req.Options[chunkerOptionName].(string)
		dopin, _ := req.Options[pinOptionName].(bool)
		rawblks, rbset := req.Options[rawLeavesOptionName].(bool)
		nocopy, _ := req.Options[noCopyOptionName].(bool)
		fscache, _ := req.Options[fstoreCacheOptionName].(bool)
		cidVer, cidVerSet := req.Options[cidVersionOptionName].(int)
		hashFunStr, _ := req.Options[hashOptionName].(string)
		inline, _ := req.Options[inlineOptionName].(bool)
		inlineLimit, _ := req.Options[inlineLimitOptionName].(int)
		toFilesStr, toFilesSet := req.Options[toFilesOptionName].(string)
		preserveMode, _ := req.Options[preserveModeOptionName].(bool)
		preserveMtime, _ := req.Options[preserveMtimeOptionName].(bool)
		mode, _ := req.Options[modeOptionName].(uint)
		mtime, _ := req.Options[mtimeOptionName].(int64)
		mtimeNsecs, _ := req.Options[mtimeNsecsOptionName].(uint)

		if chunker == "" {
			chunker = cfg.Import.UnixFSChunker.WithDefault(config.DefaultUnixFSChunker)
		}

		if hashFunStr == "" {
			hashFunStr = cfg.Import.HashFunction.WithDefault(config.DefaultHashFunction)
		}

		if !cidVerSet && !cfg.Import.CidVersion.IsDefault() {
			cidVerSet = true
			cidVer = int(cfg.Import.CidVersion.WithDefault(config.DefaultCidVersion))
		}

		if !rbset && cfg.Import.UnixFSRawLeaves != config.Default {
			rbset = true
			rawblks = cfg.Import.UnixFSRawLeaves.WithDefault(config.DefaultUnixFSRawLeaves)
		}

		// Storing optional mode or mtime (UnixFS 1.5) requires root block
		// to always be 'dag-pb' and not 'raw'. Below adjusts raw-leaves setting, if possible.
		if preserveMode || preserveMtime || mode != 0 || mtime != 0 {
			// Error if --raw-leaves flag was explicitly passed by the user.
			// (let user make a decision to manually disable it and retry)
			if rbset && rawblks {
				return fmt.Errorf("%s can't be used with UnixFS metadata like mode or modification time", rawLeavesOptionName)
			}
			// No explicit preference from user, disable raw-leaves and continue
			rbset = true
			rawblks = false
		}

		if onlyHash && toFilesSet {
			return fmt.Errorf("%s and %s options are not compatible", onlyHashOptionName, toFilesOptionName)
		}

		if wrap && toFilesSet {
			return fmt.Errorf("%s and %s options are not compatible", wrapOptionName, toFilesOptionName)
		}

		hashFunCode, ok := mh.Names[strings.ToLower(hashFunStr)]
		if !ok {
			return fmt.Errorf("unrecognized hash function: %q", strings.ToLower(hashFunStr))
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		toadd := req.Files
		if wrap {
			toadd = files.NewSliceDirectory([]files.DirEntry{
				files.FileEntry("", req.Files),
			})
		}

		opts := []options.UnixfsAddOption{
			options.Unixfs.Hash(hashFunCode),

			options.Unixfs.Inline(inline),
			options.Unixfs.InlineLimit(inlineLimit),

			options.Unixfs.Chunker(chunker),

			options.Unixfs.Pin(dopin),
			options.Unixfs.HashOnly(onlyHash),
			options.Unixfs.FsCache(fscache),
			options.Unixfs.Nocopy(nocopy),

			options.Unixfs.Progress(progress),
			options.Unixfs.Silent(silent),

			options.Unixfs.PreserveMode(preserveMode),
			options.Unixfs.PreserveMtime(preserveMtime),
		}

		if mode != 0 {
			opts = append(opts, options.Unixfs.Mode(os.FileMode(mode)))
		}

		if mtime != 0 {
			opts = append(opts, options.Unixfs.Mtime(mtime, uint32(mtimeNsecs)))
		} else if mtimeNsecs != 0 {
			return fmt.Errorf("option %q requires %q to be provided as well", mtimeNsecsOptionName, mtimeOptionName)
		}

		if cidVerSet {
			opts = append(opts, options.Unixfs.CidVersion(cidVer))
		}

		if rbset {
			opts = append(opts, options.Unixfs.RawLeaves(rawblks))
		}

		if trickle {
			opts = append(opts, options.Unixfs.Layout(options.TrickleLayout))
		}

		opts = append(opts, nil) // events option placeholder

		ipfsNode, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}
		var added int
		var fileAddedToMFS bool
		addit := toadd.Entries()
		for addit.Next() {
			_, dir := addit.Node().(files.Directory)
			errCh := make(chan error, 1)
			events := make(chan interface{}, adderOutChanSize)
			opts[len(opts)-1] = options.Unixfs.Events(events)

			go func() {
				var err error
				defer close(events)
				pathAdded, err := api.Unixfs().Add(req.Context, addit.Node(), opts...)
				if err != nil {
					errCh <- err
					return
				}

				// creating MFS pointers when optional --to-files is set
				if toFilesSet {
					if addit.Name() == "" {
						errCh <- fmt.Errorf("%s: cannot add unnamed files to MFS", toFilesOptionName)
						return
					}

					if toFilesStr == "" {
						toFilesStr = "/"
					}
					toFilesDst, err := checkPath(toFilesStr)
					if err != nil {
						errCh <- fmt.Errorf("%s: %w", toFilesOptionName, err)
						return
					}
					dstAsDir := toFilesDst[len(toFilesDst)-1] == '/'

					if dstAsDir {
						mfsNode, err := mfs.Lookup(ipfsNode.FilesRoot, toFilesDst)
						// confirm dst exists
						if err != nil {
							errCh <- fmt.Errorf("%s: MFS destination directory %q does not exist: %w", toFilesOptionName, toFilesDst, err)
							return
						}
						// confirm dst is a dir
						if mfsNode.Type() != mfs.TDir {
							errCh <- fmt.Errorf("%s: MFS destination %q is not a directory", toFilesOptionName, toFilesDst)
							return
						}
						// if MFS destination is a dir, append filename to the dir path
						toFilesDst += gopath.Base(addit.Name())
					}

					// error if we try to overwrite a preexisting file destination
					if fileAddedToMFS && !dstAsDir {
						errCh <- fmt.Errorf("%s: MFS destination is a file: only one entry can be copied to %q", toFilesOptionName, toFilesDst)
						return
					}

					_, err = mfs.Lookup(ipfsNode.FilesRoot, gopath.Dir(toFilesDst))
					if err != nil {
						errCh <- fmt.Errorf("%s: MFS destination parent %q %q does not exist: %w", toFilesOptionName, toFilesDst, gopath.Dir(toFilesDst), err)
						return
					}

					var nodeAdded ipld.Node
					nodeAdded, err = api.Dag().Get(req.Context, pathAdded.RootCid())
					if err != nil {
						errCh <- err
						return
					}
					err = mfs.PutNode(ipfsNode.FilesRoot, toFilesDst, nodeAdded)
					if err != nil {
						errCh <- fmt.Errorf("%s: cannot put node in path %q: %w", toFilesOptionName, toFilesDst, err)
						return
					}
					fileAddedToMFS = true
				}
				errCh <- err
			}()

			for event := range events {
				output, ok := event.(*coreiface.AddEvent)
				if !ok {
					return errors.New("unknown event type")
				}

				h := ""
				if (output.Path != path.ImmutablePath{}) {
					h = enc.Encode(output.Path.RootCid())
				}

				if !dir && addit.Name() != "" {
					output.Name = addit.Name()
				} else {
					output.Name = gopath.Join(addit.Name(), output.Name)
				}

				output.Mode = addit.Node().Mode()
				if ts := addit.Node().ModTime(); !ts.IsZero() {
					output.Mtime = addit.Node().ModTime().Unix()
					output.MtimeNsecs = addit.Node().ModTime().Nanosecond()
				}

				addEvent := AddEvent{
					Name:       output.Name,
					Hash:       h,
					Bytes:      output.Bytes,
					Size:       output.Size,
					Mtime:      output.Mtime,
					MtimeNsecs: output.MtimeNsecs,
				}

				if output.Mode != 0 {
					addEvent.Mode = "0" + strconv.FormatUint(uint64(output.Mode), 8)
				}

				if output.Mtime > 0 {
					addEvent.Mtime = output.Mtime
					if output.MtimeNsecs > 0 {
						addEvent.MtimeNsecs = output.MtimeNsecs
					}
				}

				if err := res.Emit(&addEvent); err != nil {
					return err
				}
			}

			if err := <-errCh; err != nil {
				return err
			}
			added++
		}

		if addit.Err() != nil {
			return addit.Err()
		}

		if added == 0 {
			return fmt.Errorf("expected a file argument")
		}

		return nil
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			sizeChan := make(chan int64, 1)
			outChan := make(chan interface{})
			req := res.Request()

			// Could be slow.
			go func() {
				size, err := req.Files.Size()
				if err != nil {
					log.Warnf("error getting files size: %s", err)
					// see comment above
					return
				}

				sizeChan <- size
			}()

			progressBar := func(wait chan struct{}) {
				defer close(wait)

				quiet, _ := req.Options[quietOptionName].(bool)
				quieter, _ := req.Options[quieterOptionName].(bool)
				quiet = quiet || quieter

				progress, _ := req.Options[progressOptionName].(bool)

				var bar *pb.ProgressBar
				if progress {
					bar = pb.New64(0).SetUnits(pb.U_BYTES)
					bar.ManualUpdate = true
					bar.ShowTimeLeft = false
					bar.ShowPercent = false
					bar.Output = os.Stderr
					bar.Start()
				}

				lastFile := ""
				lastHash := ""
				var totalProgress, prevFiles, lastBytes int64

			LOOP:
				for {
					select {
					case out, ok := <-outChan:
						if !ok {
							if quieter {
								fmt.Fprintln(os.Stdout, lastHash)
							}

							break LOOP
						}
						output := out.(*AddEvent)
						if len(output.Hash) > 0 {
							lastHash = output.Hash
							if quieter {
								continue
							}

							if progress {
								// clear progress bar line before we print "added x" output
								fmt.Fprintf(os.Stderr, "\033[2K\r")
							}
							if quiet {
								fmt.Fprintf(os.Stdout, "%s\n", output.Hash)
							} else {
								fmt.Fprintf(os.Stdout, "added %s %s\n", output.Hash, cmdenv.EscNonPrint(output.Name))
							}

						} else {
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
					case <-req.Context.Done():
						// don't set or print error here, that happens in the goroutine below
						return
					}
				}

				if progress && bar.Total == 0 && bar.Get() != 0 {
					bar.Total = bar.Get()
					bar.ShowPercent = true
					bar.ShowBar = true
					bar.ShowTimeLeft = true
					bar.Update()
				}
			}

			if e := res.Error(); e != nil {
				close(outChan)
				return e
			}

			wait := make(chan struct{})
			go progressBar(wait)

			defer func() { <-wait }()
			defer close(outChan)

			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return nil
					}

					return err
				}

				select {
				case outChan <- v:
				case <-req.Context.Done():
					return req.Context.Err()
				}
			}
		},
	},
	Type: AddEvent{},
}
