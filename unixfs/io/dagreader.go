package io

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"

	dagutils "github.com/ipfs/go-ipfs/dagutils"
	ft "github.com/ipfs/go-ipfs/unixfs"
	ftpb "github.com/ipfs/go-ipfs/unixfs/pb"
	mdag "gx/ipfs/QmRy4Qk9hbgFX9NGJRm8rBThrA8PZhNCitMgeRYyZ67s59/go-merkledag"

	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
)

// Common errors
var (
	ErrIsDir            = errors.New("this dag node is a directory")
	ErrCantReadSymlinks = errors.New("cannot currently read symlinks")
	ErrUnkownNodeType   = errors.New("unknown node type")
)

// TODO: Rename the `DagReader` interface, this doesn't read *any* DAG, just
// DAGs with UnixFS node (and it *belongs* to the `unixfs` package). Some
// alternatives: `FileReader`, `UnixFSFileReader`, `UnixFSReader`.

// A DagReader provides read-only read and seek acess to a unixfs file.
// Different implementations of readers are used for the different
// types of unixfs/protobuf-encoded nodes.
type DagReader interface {
	ReadSeekCloser
	Size() uint64
	CtxReadFull(context.Context, []byte) (int, error)
}

// A ReadSeekCloser implements interfaces to read, copy, seek and close.
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
	io.WriterTo
}

// NewDagReader creates a new reader object that reads the data represented by
// the given node, using the passed in DAGService for data retrieval
func NewDagReader(ctx context.Context, n ipld.Node, serv ipld.NodeGetter) (DagReader, error) {
	switch n := n.(type) {
	case *mdag.RawNode:
		return NewDagReaderWithSize(ctx, n, serv, uint64(len(n.RawData())))
	case *mdag.ProtoNode:
		fsNode, err := ft.FSNodeFromBytes(n.Data())
		if err != nil {
			return nil, err
		}

		switch fsNode.Type() {
		case ftpb.Data_File, ftpb.Data_Raw:
			return NewDagReaderWithSize(ctx, n, serv, fsNode.FileSize())

		case ftpb.Data_Directory, ftpb.Data_HAMTShard:
			// Dont allow reading directories
			return nil, ErrIsDir
		case ftpb.Data_Metadata:
			if len(n.Links()) == 0 {
				return nil, errors.New("incorrectly formatted metadata object")
			}
			child, err := n.Links()[0].GetNode(ctx, serv)
			if err != nil {
				return nil, err
			}

			childpb, ok := child.(*mdag.ProtoNode)
			if !ok {
				return nil, mdag.ErrNotProtobuf
			}
			return NewDagReader(ctx, childpb, serv)
		case ftpb.Data_Symlink:
			return nil, ErrCantReadSymlinks
		default:
			return nil, ft.ErrUnrecognizedType
		}
	default:
		return nil, ErrUnkownNodeType
	}
}

// NewDagReaderWithSize constructs a new `dagReader` with the file `size`
// specified.
func NewDagReaderWithSize(ctx context.Context, n ipld.Node, serv ipld.NodeGetter, size uint64) (DagReader, error) {
	//fctx, _ := context.WithCancel(ctx) // TODO: Remainder to add a cancel context.

	return &dagReader{
		ctx:          ctx,
		size:         size,
		dagTraversal: dagutils.NewTraversal(ctx, n, serv),
	}, nil
}

// dagReader provides a way to easily read the data contained in a dag.
type dagReader struct {

	// Strcuture to perform the DAG traversal, the reader just needs to
	// add logic to the visit function.
	dagTraversal *dagutils.Traversal

	// Node data buffer created from the current node being visited.
	// To avoid revisiting a buffer to complete a partial read
	// (or read after seek) store its contents here.
	nodeData *bytes.Reader

	// To satisfy the `Size()` API.
	size uint64

	// current offset for the read head within the 'file'
	offset int64

	// Our context
	ctx context.Context
}

var _ DagReader = (*dagReader)(nil)

// TODO: Why is this needed? Was there an `init` deleted?

// Size return the total length of the data from the DAG structured file.
func (dr *dagReader) Size() uint64 {
	return dr.size
}

// Read reads data from the DAG structured file. It attempts always a full
// read of the DAG until buffer is full. It uses the `Traversal` structure
// to iterate the file DAG and read every node's data into the output buffer.
func (dr *dagReader) Read(out []byte) (n int, err error) {

	// If there was a partially read buffer from the last visited node read it
	// before visiting a new one.
	if dr.nodeData != nil {
		n = dr.readDataBuffer(out)

		if n == len(out) {
			return n, nil
			// Output buffer full, no need to traverse the DAG.
		}
	}

	// Function to call on each visited node of the DAG, it fills
	// the externally-scoped `out` buffer.
	dr.dagTraversal.VisitHandler = func(node ipld.Node) error {

		// Skip internal nodes, they shouldn't have any file data
		// (see the `balanced` package for more details).
		if len(node.Links()) > 0 {
			return nil
		}

		err = dr.saveNodeData(node)
		if err != nil {
			return err
		}
		// Save the leaf node file data in a buffer in case it is only
		// partially read now and future `Read` calls reclaim the rest
		// (as each node is visited only once during `Iterate`).

		n += dr.readDataBuffer(out[n:])

		if n == len(out) {
			// Output buffer full, no need to keep traversing the DAG,
			// signal the `Traversal` to stop.
			dr.dagTraversal.Stop = true
		}

		return nil
	}

	// Iterate the DAG calling `VisitHandler` on every visited node to read
	// its data into the `out` buffer, stop if there is an error or if the
	// entire DAG is traversed (`ErrUpOnRoot`).
	err = dr.dagTraversal.Iterate()
	if err == dagutils.ErrUpOnRoot {
		return n, io.EOF
		// Reached the end of the (DAG) file, no more data to read.
	} else if err != nil {
		return n, err
		// Pass along any other errors from the `VisitHandler`.
	}

	return n, nil
}

// Load `node`'s data into the internal data buffer to later read
// it into the output buffer (`Read`) or seek into it (`Seek`).
func (dr *dagReader) saveNodeData(node ipld.Node) error {
	nodeFileData, err := ft.ReadUnixFSNodeData(node)
	if err != nil {
		return err
	}

	dr.nodeData = bytes.NewReader(nodeFileData)

	return nil
}

// Read `nodeData` into `out`. This function shouldn't have
// any errors as it's always reading from a `bytes.Reader` and
// asking only the available data in it.
func (dr *dagReader) readDataBuffer(out []byte) int {

	n, _ := dr.nodeData.Read(out)
	// Ignore the error as the EOF may not be returned in the first call,
	// explicitly ask for an empty buffer below.

	if dr.nodeData.Len() == 0 {
		dr.nodeData = nil
		// Signal that the buffer was consumed (for later `Read` calls).
		// This shouldn't return an EOF error as it's just the end of a
		// single node's data, not the entire DAG.
	}

	dr.offset += int64(n)
	// TODO: Should `offset` be incremented here or in the calling function?
	// (Doing it here saves LoC but may be confusing as it's more hidden).

	return n
}

// CtxReadFull reads data from the DAG structured file
func (dr *dagReader) CtxReadFull(ctx context.Context, b []byte) (int, error) {
	n, err := io.ReadFull(dr, b)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err
	// TODO: Unify Read and CtxReadFull (`ipfs files read`
	// is using the second one instead of the standard
	// interface), which is causing a sharness error.
}

// WriteTo writes to the given writer.
// TODO: Improve performance. It would be better to progressively
// write each node to the writer on every visit instead of allocating
// a huge buffer, that would imply defining a `VisitHandler` very similar
// to `Read` (that write to the `io.Writer` instead of the reading into
// the `bytes.Reader`). More consideration is needed to restructure those
// two `VisitHandler` to avoid repeating code.
func (dr *dagReader) WriteTo(w io.Writer) (int64, error) {
	writeBuf, err := ioutil.ReadAll(dr)
	if err != nil {
		return 0, err
	}
	return bytes.NewReader(writeBuf).WriteTo(w)
}

// Close closes the reader.
func (dr *dagReader) Close() error {
	//dr.cancel() // Reminder to cancel a context when closing.
	return nil
}

// Extract the `unixfs.FSNode` from the `ipld.Node` (assuming this
// was implemented by a `mdag.ProtoNode`).
//
// TODO: Move to the `unixfs` package.
func (dr *dagReader) extractFSNode(node ipld.Node) (*ft.FSNode, error) {
	protoNode, ok := node.(*mdag.ProtoNode)
	if !ok {
		return nil, errors.New("expected a ProtoNode as internal node")
	}

	fsNode, err := ft.FSNodeFromBytes(protoNode.Data())
	if err != nil {
		return nil, err
	}

	return fsNode, nil
}

// Seek implements `io.Seeker`, and will seek to a given offset in the file
// interface, it matches the standard unix `seek`. It moves the position of
// the `dagTraversal` and may also leave a `nodeData` buffer loaded in case
// the seek is performed to the middle of the data of a node.
//
// TODO: Support seeking from the current position (relative seek,
// `io.SeekCurrent`).
func (dr *dagReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		if offset < 0 {
			return -1, errors.New("invalid offset")
		}
		if offset == dr.offset {
			return offset, nil
		}

		left := offset
		// Amount left to seek.

		// Seek from the beginning of the DAG.
		dr.resetPosition()

		// Function to call on each visited node of the DAG which can be either
		// an internal or leaf node. In the internal node case, check the child
		// node sizes and set the corresponding index. In the leaf case, if
		// there is still an amount to seek do it inside the node's data saved
		// in the buffer, leaving it ready for a `Read` call.
		dr.dagTraversal.VisitHandler = func(node ipld.Node) error {

			if len(node.Links()) > 0 {
				// Internal node, should be a `mdag.ProtoNode` containing a UnixFS
				// `FSNode` (see the `balanced` package for more details).
				fsNode, err := dr.extractFSNode(node)
				if err != nil {
					return err
				}

				if fsNode.NumChildren() != len(node.Links()) {
					return io.EOF
					// If there aren't enough size hints don't seek.
					// https://github.com/ipfs/go-ipfs/pull/4320
					// TODO: Review this.
				}

				// Internal nodes have no data (see the `balanced` package
				// for more details), so just iterate through the sizes of
				// its children (advancing the child index of `dagTraversal`)
				// to find where we need to go down to.
				for {
					childSize := fsNode.BlockSize(int(dr.dagTraversal.ChildIndex()))

					if childSize > uint64(left) {
						// This child's data contains the position requested
						// in `offset` (the child itself may be another internal
						// node and the search would continue in that case).
						return nil
					}

					// Else, skip this child.
					left -= int64(childSize)
					err := dr.dagTraversal.Right()
					if err != nil {
						return nil
						// No more child nodes available (`ErrRightNoChild`),
						// return (ending the search, as there won't be a child
						// to go down to).
					}
				}

			} else {
				// Leaf node, seek inside its data.
				err := dr.saveNodeData(node)
				if err != nil {
					return err
				}

				_, err = dr.nodeData.Seek(left, io.SeekStart)
				if err != nil {
					return err
				}
				// In the case of a single (leaf) node, the seek would be done
				// past the end of this buffer (instead of past the available
				// child indexes through `Right` as above).
				// TODO: What's the difference?

				return nil
			}
		}

		err := dr.dagTraversal.Search()

		// TODO: Taken from https://github.com/ipfs/go-ipfs/pull/4320.
		// Return negative number if we can't figure out the file size. Using io.EOF
		// for this seems to be good(-enough) solution as it's only returned by
		// precalcNextBuf when we step out of file range.
		// This is needed for gateway to function properly
		//if err == io.EOF && dr.file.Type() == ftpb.Data_File {
		if err == io.EOF {
			return -1, nil
		}

		if err != nil {
			return 0, err
		}

		dr.offset = offset
		return dr.offset, nil

	case io.SeekCurrent:
		// TODO: This can be improved supporting relative searches
		// in the `Traversal`.

		if offset == 0 {
			return dr.offset, nil
		}

		noffset := dr.offset + offset
		return dr.Seek(noffset, io.SeekStart)
	case io.SeekEnd:
		// TODO: This might be improved adding a left movement to the
		// `Traversal`, but would it be worth it? Seeking from one end
		// (`SeekStart`) seems the same as seeking from another (`SeekEnd`).

		noffset := int64(dr.Size()) - offset
		n, err := dr.Seek(noffset, io.SeekStart)

		return n, err

	default:
		return 0, errors.New("invalid whence")
	}
}

// Reset the reader position: reset the `dagTraversal` and discard
// any partially used node's data in the `nodeData` buffer.
func (dr *dagReader) resetPosition() {
	dr.dagTraversal.ResetPosition()
	dr.nodeData = nil
}
