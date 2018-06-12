package io

import (
	"context"
	"errors"
	"fmt"
	"io"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	ftpb "github.com/ipfs/go-ipfs/unixfs/pb"

	ipld "gx/ipfs/QmWi2BYBL5gJ3CiAiQchg6rn1A8iBsrWy51EYxvHVjFvLb/go-ipld-format"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
)

// PBDagReader provides a way to easily read the data contained in a dag.
type PBDagReader struct {
	serv ipld.NodeGetter

	// the node being read
	node *mdag.ProtoNode

	// cached protobuf structure from node.Data
	pbdata *ftpb.Data

	// the current data buffer to be read from
	// will either be a bytes.Reader or a child DagReader
	buf ReadSeekCloser

	// NodePromises for each of 'nodes' child links
	promises []*ipld.NodePromise

	// the cid of each child of the current node
	links []*cid.Cid

	// the index of the child link currently being read from
	linkPosition int

	// current offset for the read head within the 'file'
	offset int64

	// Our context
	ctx context.Context

	// context cancel for children
	cancel func()
}

var _ DagReader = (*PBDagReader)(nil)

// NewPBFileReader constructs a new PBFileReader.
func NewPBFileReader(ctx context.Context, n *mdag.ProtoNode, pb *ftpb.Data, serv ipld.NodeGetter) (*PBDagReader, error) {
	err := ft.ValidatePB(n.Links(), pb)
	if err != nil {
		return nil, err
	}

	curLinks := getLinkCids(n)
	data := pb.GetData()

	fctx, cancel := context.WithCancel(ctx)
	return &PBDagReader{
		node:     n,
		serv:     serv,
		buf:      NewBufDagReader(data),
		promises: make([]*ipld.NodePromise, len(curLinks)),
		links:    curLinks,
		ctx:      fctx,
		cancel:   cancel,
		pbdata:   pb,
	}, nil
}

const preloadSize = 10

func (dr *PBDagReader) preloadNextNodes(ctx context.Context) {
	beg := dr.linkPosition
	end := beg + preloadSize
	if end >= len(dr.links) {
		end = len(dr.links)
	}

	for i, p := range ipld.GetNodes(ctx, dr.serv, dr.links[beg:end]) {
		dr.promises[beg+i] = p
	}
}

// precalcNextBuf follows the next link in line and loads it from the
// DAGService, setting the next buffer to read from
func (dr *PBDagReader) precalcNextBuf(ctx context.Context) error {
	if dr.buf != nil {
		dr.buf.Close() // Just to make sure
		dr.buf = nil
	}

	if dr.linkPosition >= len(dr.promises) {
		return io.EOF
	}

	if dr.promises[dr.linkPosition] == nil {
		dr.preloadNextNodes(ctx)
	}

	nxt, err := dr.promises[dr.linkPosition].Get(ctx)
	if err != nil {
		return err
	}
	dr.promises[dr.linkPosition] = nil
	linkPos := dr.linkPosition
	dr.linkPosition++

	switch nxt := nxt.(type) {
	case *mdag.ProtoNode:
		pb := new(ftpb.Data)
		err = proto.Unmarshal(nxt.Data(), pb)
		if err != nil {
			return fmt.Errorf("incorrectly formatted protobuf: %s", err)
		}

		switch pb.GetType() {
		case ftpb.Data_Directory, ftpb.Data_HAMTShard:
			// A directory should not exist within a file
			return ft.ErrInvalidDirLocation
		case ftpb.Data_File:
			buf, err := NewPBFileReader(dr.ctx, nxt, pb, dr.serv)
			if err != nil {
				return err
			}
			dr.buf = buf
		case ftpb.Data_Raw:
			dr.buf = NewBufDagReader(pb.GetData())
		case ftpb.Data_Metadata:
			return errors.New("shouldnt have had metadata object inside file")
		case ftpb.Data_Symlink:
			return errors.New("shouldnt have had symlink inside file")
		default:
			return ft.ErrUnrecognizedType
		}
	default:
		var err error
		dr.buf, err = NewDagReader(ctx, nxt, dr.serv)
		if err != nil {
			return err
		}
	}
	if len(dr.pbdata.Blocksizes) > 0 {
		// if blocksizes are set, ensure that we read exactly that amount of data
		dr.buf, err = newSizeAdjReadSeekCloser(dr.buf, dr.pbdata.Blocksizes[linkPos])
		if err != nil {
			return err
		}
	}
	return nil
}

func getLinkCids(n ipld.Node) []*cid.Cid {
	links := n.Links()
	out := make([]*cid.Cid, 0, len(links))
	for _, l := range links {
		out = append(out, l.Cid)
	}
	return out
}

// Size return the total length of the data from the DAG structured file.
func (dr *PBDagReader) Size() uint64 {
	return dr.pbdata.GetFilesize()
}

// Read reads data from the DAG structured file
func (dr *PBDagReader) Read(b []byte) (int, error) {
	return dr.CtxReadFull(dr.ctx, b)
}

// CtxReadFull reads data from the DAG structured file
func (dr *PBDagReader) CtxReadFull(ctx context.Context, b []byte) (int, error) {
	if dr.buf == nil {
		if err := dr.precalcNextBuf(ctx); err != nil {
			return 0, err
		}
	}

	// If no cached buffer, load one
	total := 0
	for {
		// Attempt to fill bytes from cached buffer
		n, err := io.ReadFull(dr.buf, b[total:])
		total += n
		dr.offset += int64(n)
		switch err {
		// io.EOF will happen is dr.buf had noting more to read (n == 0)
		case io.EOF, io.ErrUnexpectedEOF:
			// do nothing
		case nil:
			return total, nil
		default:
			return total, err
		}

		// if we are not done with the output buffer load next block
		err = dr.precalcNextBuf(ctx)
		if err != nil {
			return total, err
		}
	}
}

// WriteTo writes to the given writer.
func (dr *PBDagReader) WriteTo(w io.Writer) (int64, error) {
	// If no cached buffer, load one
	if dr.buf == nil {
		if err := dr.precalcNextBuf(dr.ctx); err != nil {
			return 0, err
		}
	}

	total := int64(0)
	for {
		// Attempt to write bytes from cached buffer
		n, err := dr.buf.WriteTo(w)
		total += n
		dr.offset += n
		if err != nil {
			if err != io.EOF {
				return total, err
			}
		}

		// Otherwise, load up the next block
		err = dr.precalcNextBuf(dr.ctx)
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
}

// Close closes the reader.
func (dr *PBDagReader) Close() error {
	dr.cancel()
	return nil
}

// Offset returns the current reader offset
func (dr *PBDagReader) Offset() int64 {
	return dr.offset
}

// Seek implements io.Seeker, and will seek to a given offset in the file
// interface matches standard unix seek
// TODO: check if we can do relative seeks, to reduce the amount of dagreader
// recreations that need to happen.
func (dr *PBDagReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		if offset < 0 {
			return -1, errors.New("invalid offset")
		}
		if offset == dr.offset {
			return offset, nil
		}

		// Grab cached protobuf object (solely to make code look cleaner)
		pb := dr.pbdata

		// left represents the number of bytes remaining to seek to (from beginning)
		left := offset
		if int64(len(pb.Data)) >= offset {
			// Close current buf to close potential child dagreader
			if dr.buf != nil {
				dr.buf.Close()
			}
			dr.buf = NewBufDagReader(pb.GetData()[offset:])

			// start reading links from the beginning
			dr.linkPosition = 0
			dr.offset = offset
			return offset, nil
		}

		// error out if we have links but not blocksizes
		if len(pb.Blocksizes) == 0 && len(dr.node.Links()) > 0 {
			return -1, errors.New("unseekable node (no blocksizes defined)")
		}

		// skip past root block data
		left -= int64(len(pb.Data))

		// iterate through links and find where we need to be
		for i := 0; i < len(pb.Blocksizes); i++ {
			if pb.Blocksizes[i] > uint64(left) {
				dr.linkPosition = i
				break
			} else {
				left -= int64(pb.Blocksizes[i])
			}
		}

		// start sub-block request
		err := dr.precalcNextBuf(dr.ctx)
		if err != nil {
			return 0, err
		}

		// set proper offset within child readseeker
		n, err := dr.buf.Seek(left, io.SeekStart)
		if err != nil {
			return -1, err
		}

		// sanity
		left -= n
		if left != 0 {
			return -1, errors.New("failed to seek properly")
		}
		dr.offset = offset
		return offset, nil
	case io.SeekCurrent:
		// TODO: be smarter here
		if offset == 0 {
			return dr.offset, nil
		}

		noffset := dr.offset + offset
		return dr.Seek(noffset, io.SeekStart)
	case io.SeekEnd:
		noffset := int64(dr.pbdata.GetFilesize()) - offset
		n, err := dr.Seek(noffset, io.SeekStart)

		// Return negative number if we can't figure out the file size. Using io.EOF
		// for this seems to be good(-enough) solution as it's only returned by
		// precalcNextBuf when we step out of file range.
		// This is needed for gateway to function properly
		if err == io.EOF && *dr.pbdata.Type == ftpb.Data_File {
			return -1, nil
		}
		return n, err
	default:
		return 0, errors.New("invalid whence")
	}
}
