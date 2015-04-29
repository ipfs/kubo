package tar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	gopath "path"
	"time"

	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	upb "github.com/ipfs/go-ipfs/unixfs/pb"
)

type Reader struct {
	buf        bytes.Buffer
	closed     bool
	signalChan chan struct{}
	dag        mdag.DAGService
	resolver   *path.Resolver
	writer     *tar.Writer
	gzipWriter *gzip.Writer
	err        error
}

func NewReader(path path.Path, dag mdag.DAGService, dagnode *mdag.Node, compression int) (*Reader, error) {

	reader := &Reader{
		signalChan: make(chan struct{}),
		dag:        dag,
	}

	var err error
	if compression != gzip.NoCompression {
		reader.gzipWriter, err = gzip.NewWriterLevel(&reader.buf, compression)
		if err != nil {
			return nil, err
		}
		reader.writer = tar.NewWriter(reader.gzipWriter)
	} else {
		reader.writer = tar.NewWriter(&reader.buf)
	}

	// writeToBuf will write the data to the buffer, and will signal when there
	// is new data to read
	_, filename := gopath.Split(path.String())
	go reader.writeToBuf(dagnode, filename, 0)

	return reader, nil
}

func (r *Reader) writeToBuf(dagnode *mdag.Node, path string, depth int) {
	pb := new(upb.Data)
	err := proto.Unmarshal(dagnode.Data, pb)
	if err != nil {
		r.emitError(err)
		return
	}

	if depth == 0 {
		defer r.close()
	}

	if pb.GetType() == upb.Data_Directory {
		err = r.writer.WriteHeader(&tar.Header{
			Name:     path,
			Typeflag: tar.TypeDir,
			Mode:     0777,
			ModTime:  time.Now(),
			// TODO: set mode, dates, etc. when added to unixFS
		})
		if err != nil {
			r.emitError(err)
			return
		}
		r.flush()

		ctx, cancel := context.WithTimeout(context.TODO(), time.Second*60)
		defer cancel()

		for i, ng := range r.dag.GetDAG(ctx, dagnode) {
			childNode, err := ng.Get(ctx)
			if err != nil {
				r.emitError(err)
				return
			}
			r.writeToBuf(childNode, gopath.Join(path, dagnode.Links[i].Name), depth+1)
		}
		return
	}

	err = r.writer.WriteHeader(&tar.Header{
		Name:     path,
		Size:     int64(pb.GetFilesize()),
		Typeflag: tar.TypeReg,
		Mode:     0644,
		ModTime:  time.Now(),
		// TODO: set mode, dates, etc. when added to unixFS
	})
	if err != nil {
		r.emitError(err)
		return
	}
	r.flush()

	reader, err := uio.NewDagReader(context.TODO(), dagnode, r.dag)
	if err != nil {
		r.emitError(err)
		return
	}

	err = r.syncCopy(reader)
	if err != nil {
		r.emitError(err)
		return
	}
}

func (r *Reader) Read(p []byte) (int, error) {
	// wait for the goroutine that is writing data to the buffer to tell us
	// there is something to read
	if !r.closed {
		<-r.signalChan
	}

	if r.err != nil {
		return 0, r.err
	}

	if !r.closed {
		defer r.signal()
	}

	if r.buf.Len() == 0 {
		if r.closed {
			return 0, io.EOF
		}
		return 0, nil
	}

	n, err := r.buf.Read(p)
	if err == io.EOF && !r.closed || r.buf.Len() > 0 {
		return n, nil
	}

	return n, err
}

func (r *Reader) signal() {
	r.signalChan <- struct{}{}
}

func (r *Reader) flush() {
	r.signal()
	<-r.signalChan
}

func (r *Reader) emitError(err error) {
	r.err = err
	r.signal()
}

func (r *Reader) close() {
	r.closed = true
	defer r.signal()
	err := r.writer.Close()
	if err != nil {
		r.emitError(err)
		return
	}
	if r.gzipWriter != nil {
		err = r.gzipWriter.Close()
		if err != nil {
			r.emitError(err)
			return
		}
	}
}

func (r *Reader) syncCopy(reader io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		nr, err := reader.Read(buf)
		if nr > 0 {
			_, err := r.writer.Write(buf[:nr])
			if err != nil {
				return err
			}
			r.flush()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}
