package tar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"io"
	gopath "path"
	"time"

	path "github.com/jbenet/go-ipfs/path"
	mdag "github.com/jbenet/go-ipfs/struct/merkledag"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	upb "github.com/jbenet/go-ipfs/unixfs/pb"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
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

func NewReader(path path.Path, dag mdag.DAGService, resolver *path.Resolver, compression int) (*Reader, error) {

	reader := &Reader{
		signalChan: make(chan struct{}),
		dag:        dag,
		resolver:   resolver,
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

	dagnode, err := resolver.ResolvePath(path)
	if err != nil {
		return nil, err
	}

	// writeToBuf will write the data to the buffer, and will signal when there
	// is new data to read
	_, filename := gopath.Split(path.String())
	go reader.writeToBuf(dagnode, filename, 0)

	return reader, nil
}

func (i *Reader) writeToBuf(dagnode *mdag.Node, path string, depth int) {
	pb := new(upb.Data)
	err := proto.Unmarshal(dagnode.Data, pb)
	if err != nil {
		i.emitError(err)
		return
	}

	if depth == 0 {
		defer i.close()
	}

	if pb.GetType() == upb.Data_Directory {
		err = i.writer.WriteHeader(&tar.Header{
			Name:     path,
			Typeflag: tar.TypeDir,
			Mode:     0777,
			ModTime:  time.Now(),
			// TODO: set mode, dates, etc. when added to unixFS
		})
		if err != nil {
			i.emitError(err)
			return
		}
		i.flush()

		for _, link := range dagnode.Links {
			childNode, err := link.GetNode(i.dag)
			if err != nil {
				i.emitError(err)
				return
			}
			i.writeToBuf(childNode, gopath.Join(path, link.Name), depth+1)
		}
		return
	}

	err = i.writer.WriteHeader(&tar.Header{
		Name:     path,
		Size:     int64(pb.GetFilesize()),
		Typeflag: tar.TypeReg,
		Mode:     0644,
		ModTime:  time.Now(),
		// TODO: set mode, dates, etc. when added to unixFS
	})
	if err != nil {
		i.emitError(err)
		return
	}
	i.flush()

	reader, err := uio.NewDagReader(context.TODO(), dagnode, i.dag)
	if err != nil {
		i.emitError(err)
		return
	}

	err = i.syncCopy(reader)
	if err != nil {
		i.emitError(err)
		return
	}
}

func (i *Reader) Read(p []byte) (int, error) {
	// wait for the goroutine that is writing data to the buffer to tell us
	// there is something to read
	if !i.closed {
		<-i.signalChan
	}

	if i.err != nil {
		return 0, i.err
	}

	if !i.closed {
		defer i.signal()
	}

	if i.buf.Len() == 0 {
		if i.closed {
			return 0, io.EOF
		}
		return 0, nil
	}

	n, err := i.buf.Read(p)
	if err == io.EOF && !i.closed || i.buf.Len() > 0 {
		return n, nil
	}

	return n, err
}

func (i *Reader) signal() {
	i.signalChan <- struct{}{}
}

func (i *Reader) flush() {
	i.signal()
	<-i.signalChan
}

func (i *Reader) emitError(err error) {
	i.err = err
	i.signal()
}

func (i *Reader) close() {
	i.closed = true
	defer i.signal()
	err := i.writer.Close()
	if err != nil {
		i.emitError(err)
		return
	}
	if i.gzipWriter != nil {
		err = i.gzipWriter.Close()
		if err != nil {
			i.emitError(err)
			return
		}
	}
}

func (i *Reader) syncCopy(reader io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		nr, err := reader.Read(buf)
		if nr > 0 {
			_, err := i.writer.Write(buf[:nr])
			if err != nil {
				return err
			}
			i.flush()
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
