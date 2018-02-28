// Package archive provides utilities to archive and compress a [Unixfs] DAG.
package archive

import (
	"bufio"
	"compress/gzip"
	"context"
	"io"
	"path"

	tar "github.com/ipfs/go-ipfs/unixfs/archive/tar"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// DefaultBufSize is the buffer size for gets. for now, 1MB, which is ~4 blocks.
// TODO: does this need to be configurable?
var DefaultBufSize = 1048576

type identityWriteCloser struct {
	w io.Writer
}

func (i *identityWriteCloser) Write(p []byte) (int, error) {
	return i.w.Write(p)
}

func (i *identityWriteCloser) Close() error {
	return nil
}

// DagArchive is equivalent to `ipfs getdag $hash | maybe_tar | maybe_gzip`
func DagArchive(ctx context.Context, nd ipld.Node, name string, dag ipld.DAGService, archive bool, compression int) (io.Reader, error) {

	cleaned := path.Clean(name)
	_, filename := path.Split(cleaned)

	// need to connect a writer to a reader
	piper, pipew := io.Pipe()
	checkErrAndClosePipe := func(err error) bool {
		if err != nil {
			pipew.CloseWithError(err)
			return true
		}
		return false
	}

	// use a buffered writer to parallelize task
	bufw := bufio.NewWriterSize(pipew, DefaultBufSize)

	// compression determines whether to use gzip compression.
	maybeGzw, err := newMaybeGzWriter(bufw, compression)
	if checkErrAndClosePipe(err) {
		return nil, err
	}

	closeGzwAndPipe := func() {
		if err := maybeGzw.Close(); checkErrAndClosePipe(err) {
			return
		}
		if err := bufw.Flush(); checkErrAndClosePipe(err) {
			return
		}
		pipew.Close() // everything seems to be ok.
	}

	if !archive && compression != gzip.NoCompression {
		// the case when the node is a file
		dagr, err := uio.NewDagReader(ctx, nd, dag)
		if checkErrAndClosePipe(err) {
			return nil, err
		}

		go func() {
			if _, err := dagr.WriteTo(maybeGzw); checkErrAndClosePipe(err) {
				return
			}
			closeGzwAndPipe() // everything seems to be ok
		}()
	} else {
		// the case for 1. archive, and 2. not archived and not compressed, in which tar is used anyway as a transport format

		// construct the tar writer
		w, err := tar.NewWriter(ctx, dag, archive, compression, maybeGzw)
		if checkErrAndClosePipe(err) {
			return nil, err
		}

		go func() {
			// write all the nodes recursively
			if err := w.WriteNode(nd, filename); checkErrAndClosePipe(err) {
				return
			}
			w.Close()         // close tar writer
			closeGzwAndPipe() // everything seems to be ok
		}()
	}

	return piper, nil
}

func newMaybeGzWriter(w io.Writer, compression int) (io.WriteCloser, error) {
	if compression != gzip.NoCompression {
		return gzip.NewWriterLevel(w, compression)
	}
	return &identityWriteCloser{w}, nil
}
