package commands

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	core "github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"

	"gx/ipfs/QmPtj12fdwuAqj9sBSTNUxBNu8kCGNp8b3o8yUzMm5GHpq/pb"
	tar "gx/ipfs/QmQine7gvHncNevKtG9QXxf3nXcwSj6aDDmMm52mHofEEp/tar-utils"
	uarchive "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs/archive"
	path "gx/ipfs/QmVi2uUygezqaMTqs3Yzt5FcZFHJoYD4B7jQ2BELjj7ZuY/go-path"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	dag "gx/ipfs/QmcGt25mrjuB2kKW2zhPbXVZNHc4yoTDQ65NA8m6auP2f1/go-merkledag"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var ErrInvalidCompressionLevel = errors.New("compression level must be between 1 and 9")

const (
	outputOptionName           = "output"
	archiveOptionName          = "archive"
	compressOptionName         = "compress"
	compressionLevelOptionName = "compression-level"
)

var GetCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Download IPFS objects.",
		ShortDescription: `
Stores to disk the data contained an IPFS or IPNS object(s) at the given path.

By default, the output will be stored at './<ipfs-path>', but an alternate
path can be specified with '--output=<path>' or '-o=<path>'.

To output a TAR archive instead of unpacked files, use '--archive' or '-a'.

To compress the output with GZIP compression, use '--compress' or '-C'. You
may also specify the level of compression by specifying '-l=<1-9>'.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, false, "The path to the IPFS object(s) to be outputted.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(outputOptionName, "o", "The path where the output should be stored."),
		cmdkit.BoolOption(archiveOptionName, "a", "Output a TAR archive."),
		cmdkit.BoolOption(compressOptionName, "C", "Compress the output with GZIP compression."),
		cmdkit.IntOption(compressionLevelOptionName, "l", "The level of compression (1-9)."),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		_, err := getCompressOptions(req)
		return err
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cmplvl, err := getCompressOptions(req)
		if err != nil {
			return err
		}

		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}
		p := path.Path(req.Arguments[0])
		ctx := req.Context
		dn, err := core.Resolve(ctx, node.Namesys, node.Resolver, p)
		if err != nil {
			return err
		}

		switch dn := dn.(type) {
		case *dag.ProtoNode:
			size, err := dn.Size()
			if err != nil {
				return err
			}

			res.SetLength(size)
		case *dag.RawNode:
			res.SetLength(uint64(len(dn.RawData())))
		default:
			return err
		}

		archive, _ := req.Options[archiveOptionName].(bool)
		reader, err := uarchive.DagArchive(ctx, dn, p.String(), node.DAG, archive, cmplvl)
		if err != nil {
			return err
		}

		return res.Emit(reader)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			req := res.Request()

			v, err := res.Next()
			if err != nil {
				return err
			}

			outReader, ok := v.(io.Reader)
			if !ok {
				return e.New(e.TypeErr(outReader, v))
			}

			outPath := getOutPath(req)

			cmplvl, err := getCompressOptions(req)
			if err != nil {
				return err
			}

			archive, _ := req.Options[archiveOptionName].(bool)

			gw := getWriter{
				Out:         os.Stdout,
				Err:         os.Stderr,
				Archive:     archive,
				Compression: cmplvl,
				Size:        int64(res.Length()),
			}

			return gw.Write(outReader, outPath)
		},
	},
}

type clearlineReader struct {
	io.Reader
	out io.Writer
}

func (r *clearlineReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err == io.EOF {
		// callback
		fmt.Fprintf(r.out, "\033[2K\r") // clear progress bar line on EOF
	}
	return
}

func progressBarForReader(out io.Writer, r io.Reader, l int64) (*pb.ProgressBar, io.Reader) {
	bar := makeProgressBar(out, l)
	barR := bar.NewProxyReader(r)
	return bar, &clearlineReader{barR, out}
}

func makeProgressBar(out io.Writer, l int64) *pb.ProgressBar {
	// setup bar reader
	// TODO: get total length of files
	bar := pb.New64(l).SetUnits(pb.U_BYTES)
	bar.Output = out

	// the progress bar lib doesn't give us a way to get the width of the output,
	// so as a hack we just use a callback to measure the output, then git rid of it
	bar.Callback = func(line string) {
		terminalWidth := len(line)
		bar.Callback = nil
		log.Infof("terminal width: %v\n", terminalWidth)
	}
	return bar
}

func getOutPath(req *cmds.Request) string {
	outPath, _ := req.Options[outputOptionName].(string)
	if outPath == "" {
		trimmed := strings.TrimRight(req.Arguments[0], "/")
		_, outPath = filepath.Split(trimmed)
		outPath = filepath.Clean(outPath)
	}
	return outPath
}

type getWriter struct {
	Out io.Writer // for output to user
	Err io.Writer // for progress bar output

	Archive     bool
	Compression int
	Size        int64
}

func (gw *getWriter) Write(r io.Reader, fpath string) error {
	if gw.Archive || gw.Compression != gzip.NoCompression {
		return gw.writeArchive(r, fpath)
	}
	return gw.writeExtracted(r, fpath)
}

func (gw *getWriter) writeArchive(r io.Reader, fpath string) error {
	// adjust file name if tar
	if gw.Archive {
		if !strings.HasSuffix(fpath, ".tar") && !strings.HasSuffix(fpath, ".tar.gz") {
			fpath += ".tar"
		}
	}

	// adjust file name if gz
	if gw.Compression != gzip.NoCompression {
		if !strings.HasSuffix(fpath, ".gz") {
			fpath += ".gz"
		}
	}

	// create file
	file, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(gw.Out, "Saving archive to %s\n", fpath)
	bar, barR := progressBarForReader(gw.Err, r, gw.Size)
	bar.Start()
	defer bar.Finish()

	_, err = io.Copy(file, barR)
	return err
}

func (gw *getWriter) writeExtracted(r io.Reader, fpath string) error {
	fmt.Fprintf(gw.Out, "Saving file(s) to %s\n", fpath)
	bar := makeProgressBar(gw.Err, gw.Size)
	bar.Start()
	defer bar.Finish()
	defer bar.Set64(gw.Size)

	extractor := &tar.Extractor{Path: fpath, Progress: bar.Add64}
	return extractor.Extract(r)
}

func getCompressOptions(req *cmds.Request) (int, error) {
	cmprs, _ := req.Options[compressOptionName].(bool)
	cmplvl, cmplvlFound := req.Options[compressionLevelOptionName].(int)
	switch {
	case !cmprs:
		return gzip.NoCompression, nil
	case cmprs && !cmplvlFound:
		return gzip.DefaultCompression, nil
	case cmprs && (cmplvl < 1 || cmplvl > 9):
		return gzip.NoCompression, ErrInvalidCompressionLevel
	}
	return cmplvl, nil
}
