package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	filestore "github.com/ipfs/go-ipfs/filestore"
	ft "gx/ipfs/QmQjEpRiwVvtowhq69dAtB4jhioPVFXiCcWZm9Sfgn7eqc/go-unixfs"
	dag "gx/ipfs/QmRiQCJZ91B7VNmLvA6sxzDuBJGSojS3uXHHVuNr3iueNZ/go-merkledag"
	dagtest "gx/ipfs/QmRiQCJZ91B7VNmLvA6sxzDuBJGSojS3uXHHVuNr3iueNZ/go-merkledag/test"
	blockservice "gx/ipfs/QmbSB9Uh3wVgmiCb1fAb8zuC3qAE6un4kd1jvatUurfAmB/go-blockservice"

	cmds "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	pb "gx/ipfs/QmPtj12fdwuAqj9sBSTNUxBNu8kCGNp8b3o8yUzMm5GHpq/pb"
	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	files "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit/files"
	offline "gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	mfs "gx/ipfs/QmdghKsSDa2AD1kC4qYRnVYWqZecdSBRZjeXRdhMYYhafj/go-mfs"
)

// ErrDepthLimitExceeded indicates that the max depth has been exceeded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

const (
	quietOptionName       = "quiet"
	quieterOptionName     = "quieter"
	silentOptionName      = "silent"
	progressOptionName    = "progress"
	trickleOptionName     = "trickle"
	wrapOptionName        = "wrap-with-directory"
	hiddenOptionName      = "hidden"
	onlyHashOptionName    = "only-hash"
	chunkerOptionName     = "chunker"
	pinOptionName         = "pin"
	rawLeavesOptionName   = "raw-leaves"
	noCopyOptionName      = "nocopy"
	fstoreCacheOptionName = "fscache"
	cidVersionOptionName  = "cid-version"
	hashOptionName        = "hash"
)

const adderOutChanSize = 8

var AddCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
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

The chunker option, '-s', specifies the chunking strategy that dictates
how to break files into blocks. Blocks with same content can
be deduplicated. The default is a fixed block size of
256 * 1024 bytes, 'size-262144'. Alternatively, you can use the
rabin chunker for content defined chunking by specifying
rabin-[min]-[avg]-[max] (where min/avg/max refer to the resulting
chunk sizes). Using other chunking strategies will produce
different hashes for the same file.

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
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("path", true, true, "The path to a file to be added to ipfs.").EnableRecursive().EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmds.OptionRecursivePath, // a builtin option that allows recursive paths (-r, --recursive)
		cmdkit.BoolOption(quietOptionName, "q", "Write minimal output."),
		cmdkit.BoolOption(quieterOptionName, "Q", "Write only final hash."),
		cmdkit.BoolOption(silentOptionName, "Write no output."),
		cmdkit.BoolOption(progressOptionName, "p", "Stream progress data."),
		cmdkit.BoolOption(trickleOptionName, "t", "Use trickle-dag format for dag generation."),
		cmdkit.BoolOption(onlyHashOptionName, "n", "Only chunk and hash - do not write to disk."),
		cmdkit.BoolOption(wrapOptionName, "w", "Wrap files with a directory object."),
		cmdkit.BoolOption(hiddenOptionName, "H", "Include files that are hidden. Only takes effect on recursive add."),
		cmdkit.StringOption(chunkerOptionName, "s", "Chunking algorithm, size-[bytes] or rabin-[min]-[avg]-[max]").WithDefault("size-262144"),
		cmdkit.BoolOption(pinOptionName, "Pin this object when adding.").WithDefault(true),
		cmdkit.BoolOption(rawLeavesOptionName, "Use raw blocks for leaf nodes. (experimental)"),
		cmdkit.BoolOption(noCopyOptionName, "Add the file using filestore. Implies raw-leaves. (experimental)"),
		cmdkit.BoolOption(fstoreCacheOptionName, "Check the filestore for pre-existing blocks. (experimental)"),
		cmdkit.IntOption(cidVersionOptionName, "CID version. Defaults to 0 unless an option that depends on CIDv1 is passed. (experimental)"),
		cmdkit.StringOption(hashOptionName, "Hash function to use. Implies CIDv1 if not sha2-256. (experimental)").WithDefault("sha2-256"),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		quiet, _ := req.Options[quietOptionName].(bool)
		quieter, _ := req.Options[quieterOptionName].(bool)
		quiet = quiet || quieter

		silent, _ := req.Options[silentOptionName].(bool)

		if quiet || silent {
			return nil
		}

		// ipfs cli progress bar defaults to true unless quiet or silent is used
		_, found := req.Options[progressOptionName].(bool)
		if !found {
			req.Options[progressOptionName] = true
		}

		return nil
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) {
		n, err := GetNode(env)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		cfg, err := n.Repo.Config()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		// check if repo will exceed storage limit if added
		// TODO: this doesn't handle the case if the hashed file is already in blocks (deduplicated)
		// TODO: conditional GC is disabled due to it is somehow not possible to pass the size to the daemon
		//if err := corerepo.ConditionalGC(req.Context(), n, uint64(size)); err != nil {
		//	res.SetError(err, cmdkit.ErrNormal)
		//	return
		//}

		progress, _ := req.Options[progressOptionName].(bool)
		trickle, _ := req.Options[trickleOptionName].(bool)
		wrap, _ := req.Options[wrapOptionName].(bool)
		hash, _ := req.Options[onlyHashOptionName].(bool)
		hidden, _ := req.Options[hiddenOptionName].(bool)
		silent, _ := req.Options[silentOptionName].(bool)
		chunker, _ := req.Options[chunkerOptionName].(string)
		dopin, _ := req.Options[pinOptionName].(bool)
		rawblks, rbset := req.Options[rawLeavesOptionName].(bool)
		nocopy, _ := req.Options[noCopyOptionName].(bool)
		fscache, _ := req.Options[fstoreCacheOptionName].(bool)
		cidVer, cidVerSet := req.Options[cidVersionOptionName].(int)
		hashFunStr, _ := req.Options[hashOptionName].(string)

		// The arguments are subject to the following constraints.
		//
		// nocopy -> filestoreEnabled
		// nocopy -> rawblocks
		// (hash != sha2-256) -> cidv1

		// NOTE: 'rawblocks -> cidv1' is missing. Legacy reasons.

		// nocopy -> filestoreEnabled
		if nocopy && !cfg.Experimental.FilestoreEnabled {
			res.SetError(filestore.ErrFilestoreNotEnabled, cmdkit.ErrClient)
			return
		}

		// nocopy -> rawblocks
		if nocopy && !rawblks {
			// fixed?
			if rbset {
				res.SetError(
					fmt.Errorf("nocopy option requires '--raw-leaves' to be enabled as well"),
					cmdkit.ErrNormal,
				)
				return
			}
			// No, satisfy mandatory constraint.
			rawblks = true
		}

		// (hash != "sha2-256") -> CIDv1
		if hashFunStr != "sha2-256" && cidVer == 0 {
			if cidVerSet {
				res.SetError(
					errors.New("CIDv0 only supports sha2-256"),
					cmdkit.ErrClient,
				)
				return
			}
			cidVer = 1
		}

		// cidV1 -> raw blocks (by default)
		if cidVer > 0 && !rbset {
			rawblks = true
		}

		prefix, err := dag.PrefixForCidVersion(cidVer)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		hashFunCode, ok := mh.Names[strings.ToLower(hashFunStr)]
		if !ok {
			res.SetError(fmt.Errorf("unrecognized hash function: %s", strings.ToLower(hashFunStr)), cmdkit.ErrNormal)
			return
		}

		prefix.MhType = hashFunCode
		prefix.MhLength = -1

		if hash {
			nilnode, err := core.NewNode(n.Context(), &core.BuildCfg{
				//TODO: need this to be true or all files
				// hashed will be stored in memory!
				NilRepo: true,
			})
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			n = nilnode
		}

		addblockstore := n.Blockstore
		if !(fscache || nocopy) {
			addblockstore = bstore.NewGCBlockstore(n.BaseBlocks, n.GCLocker)
		}

		exch := n.Exchange
		local, _ := req.Options["local"].(bool)
		if local {
			exch = offline.Exchange(addblockstore)
		}

		bserv := blockservice.New(addblockstore, exch) // hash security 001
		dserv := dag.NewDAGService(bserv)

		outChan := make(chan interface{}, adderOutChanSize)

		fileAdder, err := coreunix.NewAdder(req.Context, n.Pinning, n.Blockstore, dserv)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
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
		fileAdder.NoCopy = nocopy
		fileAdder.CidBuilder = &prefix

		if hash {
			md := dagtest.Mock()
			emptyDirNode := ft.EmptyDirNode()
			// Use the same prefix for the "empty" MFS root as for the file adder.
			emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
			mr, err := mfs.NewRoot(req.Context, md, emptyDirNode, nil)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
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

		errCh := make(chan error)
		go func() {
			var err error
			defer func() { errCh <- err }()
			defer close(outChan)
			err = addAllAndPin(req.Files)
		}()

		defer res.Close()

		err = res.Emit(outChan)
		if err != nil {
			log.Error(err)
			return
		}
		err = <-errCh
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
		}
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(req *cmds.Request, re cmds.ResponseEmitter) cmds.ResponseEmitter {
			reNext, res := cmds.NewChanResponsePair(req)
			outChan := make(chan interface{})

			sizeChan := make(chan int64, 1)

			sizeFile, ok := req.Files.(files.SizeFile)
			if ok {
				// Could be slow.
				go func() {
					size, err := sizeFile.Size()
					if err != nil {
						log.Warningf("error getting files size: %s", err)
						// see comment above
						return
					}

					sizeChan <- size
				}()
			} else {
				// we don't need to error, the progress bar just
				// won't know how big the files are
				log.Warning("cannot determine size of input file")
			}

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
						output := out.(*coreunix.AddedObject)
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
								fmt.Fprintf(os.Stdout, "added %s %s\n", output.Hash, output.Name)
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
			}

			go func() {
				// defer order important! First close outChan, then wait for output to finish, then close re
				defer re.Close()

				if e := res.Error(); e != nil {
					defer close(outChan)
					re.SetError(e.Message, e.Code)
					return
				}

				wait := make(chan struct{})
				go progressBar(wait)

				defer func() { <-wait }()
				defer close(outChan)

				for {
					v, err := res.Next()
					if !cmds.HandleError(err, res, re) {
						break
					}

					select {
					case outChan <- v:
					case <-req.Context.Done():
						re.SetError(req.Context.Err(), cmdkit.ErrNormal)
						return
					}
				}
			}()

			return reNext
		},
	},
	Type: coreunix.AddedObject{},
}
