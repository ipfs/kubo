package archive

import (
	"bufio"
	"compress/gzip"
	"io"
	"path"

	cxt "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	tar "github.com/ipfs/go-ipfs/unixfs/archive/tar"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
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
func DagArchive(ctx cxt.Context, nd *mdag.Node, name string, dag mdag.DAGService, archive bool, compression int) (io.Reader, error) {

	_, filename := path.Split(name)

	// need to connect a writer to a reader
	piper, pipew := io.Pipe()

	// use a buffered writer to parallelize task
	bufw := bufio.NewWriterSize(pipew, DefaultBufSize)

	// compression determines whether to use gzip compression.
	var maybeGzw io.WriteCloser
	var err error
	if compression != gzip.NoCompression {
		maybeGzw, err = gzip.NewWriterLevel(bufw, compression)
		if err != nil {
			pipew.CloseWithError(err)
			return nil, err
		}
	} else {
		maybeGzw = &identityWriteCloser{bufw}
	}

	if !archive && compression != gzip.NoCompression {
		// the case when the node is a file
		dagr, err := uio.NewDagReader(ctx, nd, dag)
		if err != nil {
			pipew.CloseWithError(err)
			return nil, err
		}

		go func() {
			if _, err := dagr.WriteTo(maybeGzw); err != nil {
				pipew.CloseWithError(err)
				return
			}
			maybeGzw.Close()
			if err := bufw.Flush(); err != nil {
				pipew.CloseWithError(err)
				return
			}
			pipew.Close() // everything seems to be ok.
		}()
	} else {
		// the case for 1. archive, and 2. not archived and not compressed, in which tar is used anyway as a transport format

		// construct the tar writer
		w, err := tar.NewWriter(ctx, dag, archive, compression, maybeGzw)
		if err != nil {
			return nil, err
		}

		go func() {
			// write all the nodes recursively
			if err := w.WriteNode(nd, filename); err != nil {
				pipew.CloseWithError(err)
				return
			}
			w.Close()
			maybeGzw.Close()
			if err := bufw.Flush(); err != nil {
				pipew.CloseWithError(err)
				return
			}
			pipew.Close() // everything seems to be ok.
		}()
	}

	return piper, nil
}
