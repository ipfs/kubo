package io

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	ftpb "github.com/jbenet/go-ipfs/unixfs/pb"
	u "github.com/jbenet/go-ipfs/util"
)

var ErrIsDir = errors.New("this dag node is a directory")

// DagReader provides a way to easily read the data contained in a dag.
type DagReader struct {
	serv         mdag.DAGService
	node         *mdag.Node
	buf          ReadSeekCloser
	fetchChan    <-chan *mdag.Node
	linkPosition int
	offset       int64

	// Our context
	ctx context.Context

	// Context for children
	fctx   context.Context
	cancel func()
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// NewDagReader creates a new reader object that reads the data represented by the given
// node, using the passed in DAGService for data retreival
func NewDagReader(ctx context.Context, n *mdag.Node, serv mdag.DAGService) (ReadSeekCloser, error) {
	pb := new(ftpb.Data)
	err := proto.Unmarshal(n.Data, pb)
	if err != nil {
		return nil, err
	}

	switch pb.GetType() {
	case ftpb.Data_Directory:
		// Dont allow reading directories
		return nil, ErrIsDir
	case ftpb.Data_File:
		fctx, cancel := context.WithCancel(ctx)
		fetchChan := serv.GetDAG(fctx, n)
		return &DagReader{
			node:      n,
			serv:      serv,
			buf:       NewRSNCFromBytes(pb.GetData()),
			fetchChan: fetchChan,
			ctx:       ctx,
			fctx:      fctx,
			cancel:    cancel,
		}, nil
	case ftpb.Data_Raw:
		// Raw block will just be a single level, return a byte buffer
		return NewRSNCFromBytes(pb.GetData()), nil
	default:
		return nil, ft.ErrUnrecognizedType
	}
}

// precalcNextBuf follows the next link in line and loads it from the DAGService,
// setting the next buffer to read from
func (dr *DagReader) precalcNextBuf() error {
	var nxt *mdag.Node
	var ok bool

	if dr.fetchChan == nil {
		// This panic is appropriate because the select statement
		// will not panic if you try and read from a nil channel
		// it will simply hang.
		panic("fetchChan should NOT be nil")
	}
	select {
	case nxt, ok = <-dr.fetchChan:
		if !ok {
			return io.EOF
		}
	case <-dr.ctx.Done():
		return dr.ctx.Err()
	}

	pb := new(ftpb.Data)
	err := proto.Unmarshal(nxt.Data, pb)
	if err != nil {
		return err
	}

	switch pb.GetType() {
	case ftpb.Data_Directory:
		// A directory should not exist within a file
		return ft.ErrInvalidDirLocation
	case ftpb.Data_File:
		//TODO: this *should* work, needs testing first
		log.Warning("Running untested code for multilayered indirect FS reads.")
		subr, err := NewDagReader(dr.fctx, nxt, dr.serv)
		if err != nil {
			return err
		}
		dr.buf = subr
		return nil
	case ftpb.Data_Raw:
		dr.buf = NewRSNCFromBytes(pb.GetData())
		return nil
	default:
		return ft.ErrUnrecognizedType
	}
}

func (dr *DagReader) resetBlockFetch(nlinkpos int) {
	dr.cancel()
	dr.fetchChan = nil
	dr.linkPosition = nlinkpos

	var keys []u.Key
	for _, lnk := range dr.node.Links[dr.linkPosition:] {
		keys = append(keys, u.Key(lnk.Hash))
	}

	fctx, cancel := context.WithCancel(dr.ctx)
	dr.cancel = cancel
	dr.fctx = fctx
	fch := dr.serv.GetNodes(fctx, keys)
	dr.fetchChan = fch
}

// Read reads data from the DAG structured file
func (dr *DagReader) Read(b []byte) (int, error) {
	// If no cached buffer, load one
	if dr.buf == nil {
		err := dr.precalcNextBuf()
		if err != nil {
			return 0, err
		}
	}
	total := 0
	for {
		// Attempt to fill bytes from cached buffer
		n, err := dr.buf.Read(b[total:])
		total += n
		dr.offset += int64(n)
		if err != nil {
			// EOF is expected
			if err != io.EOF {
				return total, err
			}
		}

		// If weve read enough bytes, return
		if total == len(b) {
			return total, nil
		}

		// Otherwise, load up the next block
		err = dr.precalcNextBuf()
		if err != nil {
			return total, err
		}
	}
}

func (dr *DagReader) Close() error {
	if dr.fctx != nil {
		dr.cancel()
	}
	return nil
}

func (dr *DagReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case os.SEEK_SET:
		if offset < 0 {
			return -1, errors.New("Invalid offset")
		}
		//TODO: this pb should be cached
		pb := new(ftpb.Data)
		err := proto.Unmarshal(dr.node.Data, pb)
		if err != nil {
			return -1, err
		}

		if offset == 0 {
			dr.resetBlockFetch(0)
			dr.buf = NewRSNCFromBytes(pb.GetData())
			return 0, nil
		}

		left := offset
		if int64(len(pb.Data)) > offset {
			dr.buf = NewRSNCFromBytes(pb.GetData()[offset:])
			dr.linkPosition = 0
			dr.offset = offset
			return offset, nil
		} else {
			left -= int64(len(pb.Data))
		}

		i := 0
		for ; i < len(pb.Blocksizes); i++ {
			if pb.Blocksizes[i] > uint64(left) {
				break
			} else {
				left -= int64(pb.Blocksizes[i])
			}
		}
		dr.resetBlockFetch(i)
		err = dr.precalcNextBuf()
		if err != nil {
			return 0, err
		}

		n, err := io.CopyN(ioutil.Discard, dr.buf, left)
		if err != nil {
			fmt.Printf("the copy failed: %s - [%d]\n", err, n)
			return -1, err
		}
		left -= n
		if left != 0 {
			return -1, errors.New("failed to seek properly")
		}
		dr.offset = offset
		return offset, nil
	case os.SEEK_CUR:
		noffset := dr.offset + offset
		return dr.Seek(noffset, os.SEEK_SET)
	case os.SEEK_END:
		pb := new(ftpb.Data)
		err := proto.Unmarshal(dr.node.Data, pb)
		if err != nil {
			return -1, err
		}
		noffset := int64(pb.GetFilesize()) - offset
		return dr.Seek(noffset, os.SEEK_SET)
	default:
		return 0, errors.New("invalid whence")
	}
	return 0, nil
}

type readSeekNopCloser struct {
	*bytes.Reader
}

func NewRSNCFromBytes(b []byte) ReadSeekCloser {
	return &readSeekNopCloser{bytes.NewReader(b)}
}

func (r *readSeekNopCloser) Close() error { return nil }
