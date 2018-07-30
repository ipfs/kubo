package io

import (
	"context"
	"errors"
	"fmt"
	"io"

	mdag "gx/ipfs/QmRy4Qk9hbgFX9NGJRm8rBThrA8PZhNCitMgeRYyZ67s59/go-merkledag"
	ft "gx/ipfs/QmSaz8Qg77gGqvDvLKeSAY7ivDEnramSWF6T7TcRwFpHtP/go-unixfs"
	ftpb "gx/ipfs/QmSaz8Qg77gGqvDvLKeSAY7ivDEnramSWF6T7TcRwFpHtP/go-unixfs/pb"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
)

// PBDagReader provides a way to easily read the data contained in a dag.
type PBDagReader struct {
	serv ipld.NodeGetter

	// UnixFS file (it should be of type `Data_File` or `Data_Raw` only).
	file *ft.FSNode

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
func NewPBFileReader(ctx context.Context, n *mdag.ProtoNode, file *ft.FSNode, serv ipld.NodeGetter) *PBDagReader {
	fctx, cancel := context.WithCancel(ctx)
	curLinks := getLinkCids(n)
	return &PBDagReader{
		serv:     serv,
		buf:      NewBufDagReader(file.Data()),
		promises: make([]*ipld.NodePromise, len(curLinks)),
		links:    curLinks,
		ctx:      fctx,
		cancel:   cancel,
		file:     file,
	}
}

const preloadSize = 10

func (dr *PBDagReader) preload(ctx context.Context, beg int) {
	end := beg + preloadSize
	if end >= len(dr.links) {
		end = len(dr.links)
	}

	copy(dr.promises[beg:], ipld.GetNodes(ctx, dr.serv, dr.links[beg:end]))
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

	// If we drop to <= preloadSize/2 preloading nodes, preload the next 10.
	for i := dr.linkPosition; i < dr.linkPosition+preloadSize/2 && i < len(dr.promises); i++ {
		// TODO: check if canceled.
		if dr.promises[i] == nil {
			dr.preload(ctx, i)
			break
		}
	}

	nxt, err := dr.promises[dr.linkPosition].Get(ctx)
	dr.promises[dr.linkPosition] = nil
	switch err {
	case nil:
	case context.DeadlineExceeded, context.Canceled:
		err = ctx.Err()
		if err != nil {
			return ctx.Err()
		}
		// In this case, the context used to *preload* the node has been canceled.
		// We need to retry the load with our context and we might as
		// well preload some extra nodes while we're at it.
		//
		// Note: When using `Read`, this code will never execute as
		// `Read` will use the global context. It only runs if the user
		// explicitly reads with a custom context (e.g., by calling
		// `CtxReadFull`).
		dr.preload(ctx, dr.linkPosition)
		nxt, err = dr.promises[dr.linkPosition].Get(ctx)
		dr.promises[dr.linkPosition] = nil
		if err != nil {
			return err
		}
	default:
		return err
	}

	dr.linkPosition++

	return dr.loadBufNode(nxt)
}

func (dr *PBDagReader) loadBufNode(node ipld.Node) error {
	switch node := node.(type) {
	case *mdag.ProtoNode:
		fsNode, err := ft.FSNodeFromBytes(node.Data())
		if err != nil {
			return fmt.Errorf("incorrectly formatted protobuf: %s", err)
		}

		switch fsNode.Type() {
		case ftpb.Data_File:
			dr.buf = NewPBFileReader(dr.ctx, node, fsNode, dr.serv)
			return nil
		case ftpb.Data_Raw:
			dr.buf = NewBufDagReader(fsNode.Data())
			return nil
		default:
			return fmt.Errorf("found %s node in unexpected place", fsNode.Type().String())
		}
	case *mdag.RawNode:
		dr.buf = NewBufDagReader(node.RawData())
		return nil
	default:
		return ErrUnkownNodeType
	}
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
	return dr.file.FileSize()
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
	if dr.buf == nil {
		if err := dr.precalcNextBuf(dr.ctx); err != nil {
			return 0, err
		}
	}

	// If no cached buffer, load one
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

		// left represents the number of bytes remaining to seek to (from beginning)
		left := offset
		if int64(len(dr.file.Data())) >= offset {
			// Close current buf to close potential child dagreader
			if dr.buf != nil {
				dr.buf.Close()
			}
			dr.buf = NewBufDagReader(dr.file.Data()[offset:])

			// start reading links from the beginning
			dr.linkPosition = 0
			dr.offset = offset
			return offset, nil
		}

		// skip past root block data
		left -= int64(len(dr.file.Data()))

		// iterate through links and find where we need to be
		for i := 0; i < dr.file.NumChildren(); i++ {
			if dr.file.BlockSize(i) > uint64(left) {
				dr.linkPosition = i
				break
			} else {
				left -= int64(dr.file.BlockSize(i))
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
		noffset := int64(dr.file.FileSize()) - offset
		n, err := dr.Seek(noffset, io.SeekStart)

		// Return negative number if we can't figure out the file size. Using io.EOF
		// for this seems to be good(-enough) solution as it's only returned by
		// precalcNextBuf when we step out of file range.
		// This is needed for gateway to function properly
		if err == io.EOF && dr.file.Type() == ftpb.Data_File {
			return -1, nil
		}
		return n, err
	default:
		return 0, errors.New("invalid whence")
	}
}
