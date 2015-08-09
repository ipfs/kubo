package tar

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"time"

	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	cxt "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	upb "github.com/ipfs/go-ipfs/unixfs/pb"
)

const tarBlockSize = 512

// DefaultBufSize is the buffer size for gets. for now, 1MB, which is ~4 blocks.
// TODO: does this need to be configurable?
var DefaultBufSize = 1048576

func DagArchive(ctx cxt.Context, nd *mdag.Node, name string, dag mdag.DAGService, compression int) (io.Reader, error) {

	_, filename := path.Split(name)

	// need to connect a writer to a reader
	piper, pipew := io.Pipe()

	// use a buffered writer to parallelize task
	bufw := bufio.NewWriterSize(pipew, DefaultBufSize)

	// construct the tar writer
	w, err := NewWriter(bufw, dag, compression)
	if err != nil {
		return nil, err
	}

	// write all the nodes recursively
	go func() {
		if err := w.WriteNode(ctx, nd, filename); err != nil {
			pipew.CloseWithError(err)
			return
		}

		if err := bufw.Flush(); err != nil {
			pipew.CloseWithError(err)
			return
		}

		pipew.Close() // everything seems to be ok.
	}()

	return piper, nil
}

func GetTarSize(ctx cxt.Context, nd *mdag.Node, dag mdag.DAGService) (uint64, error) {
	return _getTarSize(ctx, nd, dag, true)
}

func _getTarSize(ctx cxt.Context, nd *mdag.Node, dag mdag.DAGService, isRoot bool) (uint64, error) {
	size := uint64(0)
	pb := new(upb.Data)
	if err := proto.Unmarshal(nd.Data, pb); err != nil {
		return 0, err
	}

	switch pb.GetType() {
	case upb.Data_Directory:
		for _, ng := range dag.GetDAG(ctx, nd) {
			child, err := ng.Get(ctx)
			if err != nil {
				return 0, err
			}
			childSize, err := _getTarSize(ctx, child, dag, false)
			if err != nil {
				return 0, err
			}
			size += childSize
		}
	case upb.Data_File:
		unixSize := pb.GetFilesize()
		// tar header + file size + round up to nearest 512 bytes
		size = tarBlockSize + unixSize + (tarBlockSize - unixSize%tarBlockSize)
	default:
		return 0, fmt.Errorf("unixfs type not supported: %s", pb.GetType())
	}

	if isRoot {
		size += 2 * tarBlockSize // tar root padding
	}

	return size, nil
}

// Writer is a utility structure that helps to write
// unixfs merkledag nodes as a tar archive format.
// It wraps any io.Writer.
type Writer struct {
	Dag  mdag.DAGService
	TarW *tar.Writer
}

// NewWriter wraps given io.Writer.
// compression determines whether to use gzip compression.
func NewWriter(w io.Writer, dag mdag.DAGService, compression int) (*Writer, error) {

	if compression != gzip.NoCompression {
		var err error
		w, err = gzip.NewWriterLevel(w, compression)
		if err != nil {
			return nil, err
		}
	}

	return &Writer{
		Dag:  dag,
		TarW: tar.NewWriter(w),
	}, nil
}

func (w *Writer) WriteNode(ctx cxt.Context, nd *mdag.Node, fpath string) error {
	pb := new(upb.Data)
	if err := proto.Unmarshal(nd.Data, pb); err != nil {
		return err
	}

	switch pb.GetType() {
	case upb.Data_Directory:
		return w.writeDir(ctx, nd, fpath)
	case upb.Data_File:
		return w.writeFile(ctx, nd, pb, fpath)
	default:
		return fmt.Errorf("unixfs type not supported: %s", pb.GetType())
	}
}

func (w *Writer) Close() error {
	return w.TarW.Close()
}

func (w *Writer) writeFile(ctx cxt.Context, nd *mdag.Node, pb *upb.Data, fpath string) error {
	if err := writeFileHeader(w.TarW, fpath, pb.GetFilesize()); err != nil {
		return err
	}

	dagr, err := uio.NewDagReader(ctx, nd, w.Dag)
	if err != nil {
		return err
	}

	_, err = io.Copy(w.TarW, dagr)
	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (w *Writer) writeDir(ctx cxt.Context, nd *mdag.Node, fpath string) error {
	if err := writeDirHeader(w.TarW, fpath); err != nil {
		return err
	}

	for i, ng := range w.Dag.GetDAG(ctx, nd) {
		child, err := ng.Get(ctx)
		if err != nil {
			return err
		}

		npath := path.Join(fpath, nd.Links[i].Name)
		if err := w.WriteNode(ctx, child, npath); err != nil {
			return err
		}
	}

	return nil
}

func writeDirHeader(w *tar.Writer, fpath string) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Typeflag: tar.TypeDir,
		Mode:     0777,
		ModTime:  time.Now(),
		// TODO: set mode, dates, etc. when added to unixFS
	})
}

func writeFileHeader(w *tar.Writer, fpath string, size uint64) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Size:     int64(size),
		Typeflag: tar.TypeReg,
		Mode:     0644,
		ModTime:  time.Now(),
		// TODO: set mode, dates, etc. when added to unixFS
	})
}
