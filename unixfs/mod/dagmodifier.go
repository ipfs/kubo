package mod

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"

	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	help "github.com/ipfs/go-ipfs/importer/helpers"
	trickle "github.com/ipfs/go-ipfs/importer/trickle"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
	node "gx/ipfs/QmYDscK7dmdo2GZ9aumS8s5auUUAH5mR1jvj5pYhWusfK7/go-ipld-node"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
)

var ErrSeekFail = errors.New("failed to seek properly")
var ErrUnrecognizedWhence = errors.New("unrecognized whence")

// 2MB
var writebufferSize = 1 << 21

var log = logging.Logger("dagio")

// DagModifier is the only struct licensed and able to correctly
// perform surgery on a DAG 'file'
// Dear god, please rename this to something more pleasant
type DagModifier struct {
	dagserv mdag.DAGService
	curNode *mdag.ProtoNode

	splitter   chunk.SplitterGen
	ctx        context.Context
	readCancel func()

	writeStart uint64
	curWrOff   uint64
	wrBuf      *bytes.Buffer

	read uio.DagReader
}

func NewDagModifier(ctx context.Context, from node.Node, serv mdag.DAGService, spl chunk.SplitterGen) (*DagModifier, error) {
	pbn, ok := from.(*mdag.ProtoNode)
	if !ok {
		return nil, mdag.ErrNotProtobuf
	}

	return &DagModifier{
		curNode:  pbn.Copy().(*mdag.ProtoNode),
		dagserv:  serv,
		splitter: spl,
		ctx:      ctx,
	}, nil
}

// WriteAt will modify a dag file in place
func (dm *DagModifier) WriteAt(b []byte, offset int64) (int, error) {
	// TODO: this is currently VERY inneficient
	// each write that happens at an offset other than the current one causes a
	// flush to disk, and dag rewrite
	if offset == int64(dm.writeStart) && dm.wrBuf != nil {
		// If we would overwrite the previous write
		if len(b) >= dm.wrBuf.Len() {
			dm.wrBuf.Reset()
		}
	} else if uint64(offset) != dm.curWrOff {
		size, err := dm.Size()
		if err != nil {
			return 0, err
		}
		if offset > size {
			err := dm.expandSparse(offset - size)
			if err != nil {
				return 0, err
			}
		}

		err = dm.Sync()
		if err != nil {
			return 0, err
		}
		dm.writeStart = uint64(offset)
	}

	return dm.Write(b)
}

// A reader that just returns zeros
type zeroReader struct{}

func (zr zeroReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 0
	}
	return len(b), nil
}

// expandSparse grows the file with zero blocks of 4096
// A small blocksize is chosen to aid in deduplication
func (dm *DagModifier) expandSparse(size int64) error {
	r := io.LimitReader(zeroReader{}, size)
	spl := chunk.NewSizeSplitter(r, 4096)
	nnode, err := dm.appendData(dm.curNode, spl)
	if err != nil {
		return err
	}
	_, err = dm.dagserv.Add(nnode)
	if err != nil {
		return err
	}

	pbnnode, ok := nnode.(*mdag.ProtoNode)
	if !ok {
		return mdag.ErrNotProtobuf
	}

	dm.curNode = pbnnode
	return nil
}

// Write continues writing to the dag at the current offset
func (dm *DagModifier) Write(b []byte) (int, error) {
	if dm.read != nil {
		dm.read = nil
	}
	if dm.wrBuf == nil {
		dm.wrBuf = new(bytes.Buffer)
	}

	n, err := dm.wrBuf.Write(b)
	if err != nil {
		return n, err
	}
	dm.curWrOff += uint64(n)
	if dm.wrBuf.Len() > writebufferSize {
		err := dm.Sync()
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (dm *DagModifier) Size() (int64, error) {
	pbn, err := ft.FromBytes(dm.curNode.Data())
	if err != nil {
		return 0, err
	}

	if dm.wrBuf != nil {
		if uint64(dm.wrBuf.Len())+dm.writeStart > pbn.GetFilesize() {
			return int64(dm.wrBuf.Len()) + int64(dm.writeStart), nil
		}
	}

	return int64(pbn.GetFilesize()), nil
}

// Sync writes changes to this dag to disk
func (dm *DagModifier) Sync() error {
	// No buffer? Nothing to do
	if dm.wrBuf == nil {
		return nil
	}

	// If we have an active reader, kill it
	if dm.read != nil {
		dm.read = nil
		dm.readCancel()
	}

	// Number of bytes we're going to write
	buflen := dm.wrBuf.Len()

	// overwrite existing dag nodes
	thisc, done, err := dm.modifyDag(dm.curNode, dm.writeStart, dm.wrBuf)
	if err != nil {
		return err
	}

	nd, err := dm.dagserv.Get(dm.ctx, thisc)
	if err != nil {
		return err
	}

	pbnd, ok := nd.(*mdag.ProtoNode)
	if !ok {
		return mdag.ErrNotProtobuf
	}

	dm.curNode = pbnd

	// need to write past end of current dag
	if !done {
		nd, err := dm.appendData(dm.curNode, dm.splitter(dm.wrBuf))
		if err != nil {
			return err
		}

		_, err = dm.dagserv.Add(nd)
		if err != nil {
			return err
		}

		pbnode, ok := nd.(*mdag.ProtoNode)
		if !ok {
			return mdag.ErrNotProtobuf
		}

		dm.curNode = pbnode
	}

	dm.writeStart += uint64(buflen)

	dm.wrBuf = nil
	return nil
}

// modifyDag writes the data in 'data' over the data in 'node' starting at 'offset'
// returns the new key of the passed in node and whether or not all the data in the reader
// has been consumed.
func (dm *DagModifier) modifyDag(node *mdag.ProtoNode, offset uint64, data io.Reader) (*cid.Cid, bool, error) {
	f, err := ft.FromBytes(node.Data())
	if err != nil {
		return nil, false, err
	}

	// If we've reached a leaf node.
	if len(node.Links()) == 0 {
		n, err := data.Read(f.Data[offset:])
		if err != nil && err != io.EOF {
			return nil, false, err
		}

		// Update newly written node..
		b, err := proto.Marshal(f)
		if err != nil {
			return nil, false, err
		}

		nd := new(mdag.ProtoNode)
		nd.SetData(b)
		k, err := dm.dagserv.Add(nd)
		if err != nil {
			return nil, false, err
		}

		// Hey look! we're done!
		var done bool
		if n < len(f.Data[offset:]) {
			done = true
		}

		return k, done, nil
	}

	var cur uint64
	var done bool
	for i, bs := range f.GetBlocksizes() {
		// We found the correct child to write into
		if cur+bs > offset {
			child, err := node.Links()[i].GetNode(dm.ctx, dm.dagserv)
			if err != nil {
				return nil, false, err
			}

			childpb, ok := child.(*mdag.ProtoNode)
			if !ok {
				return nil, false, mdag.ErrNotProtobuf
			}

			k, sdone, err := dm.modifyDag(childpb, offset-cur, data)
			if err != nil {
				return nil, false, err
			}

			offset += bs
			node.Links()[i].Cid = k

			// Recache serialized node
			_, err = node.EncodeProtobuf(true)
			if err != nil {
				return nil, false, err
			}

			if sdone {
				// No more bytes to write!
				done = true
				break
			}
			offset = cur + bs
		}
		cur += bs
	}

	k, err := dm.dagserv.Add(node)
	return k, done, err
}

// appendData appends the blocks from the given chan to the end of this dag
func (dm *DagModifier) appendData(node *mdag.ProtoNode, spl chunk.Splitter) (node.Node, error) {
	dbp := &help.DagBuilderParams{
		Dagserv:  dm.dagserv,
		Maxlinks: help.DefaultLinksPerBlock,
	}

	return trickle.TrickleAppend(dm.ctx, node, dbp.New(spl))
}

// Read data from this dag starting at the current offset
func (dm *DagModifier) Read(b []byte) (int, error) {
	err := dm.readPrep()
	if err != nil {
		return 0, err
	}

	n, err := dm.read.Read(b)
	dm.curWrOff += uint64(n)
	return n, err
}

func (dm *DagModifier) readPrep() error {
	err := dm.Sync()
	if err != nil {
		return err
	}

	if dm.read == nil {
		ctx, cancel := context.WithCancel(dm.ctx)
		dr, err := uio.NewDagReader(ctx, dm.curNode, dm.dagserv)
		if err != nil {
			cancel()
			return err
		}

		i, err := dr.Seek(int64(dm.curWrOff), os.SEEK_SET)
		if err != nil {
			cancel()
			return err
		}

		if i != int64(dm.curWrOff) {
			cancel()
			return ErrSeekFail
		}

		dm.readCancel = cancel
		dm.read = dr
	}

	return nil
}

// Read data from this dag starting at the current offset
func (dm *DagModifier) CtxReadFull(ctx context.Context, b []byte) (int, error) {
	err := dm.readPrep()
	if err != nil {
		return 0, err
	}

	n, err := dm.read.CtxReadFull(ctx, b)
	dm.curWrOff += uint64(n)
	return n, err
}

// GetNode gets the modified DAG Node
func (dm *DagModifier) GetNode() (*mdag.ProtoNode, error) {
	err := dm.Sync()
	if err != nil {
		return nil, err
	}
	return dm.curNode.Copy().(*mdag.ProtoNode), nil
}

// HasChanges returned whether or not there are unflushed changes to this dag
func (dm *DagModifier) HasChanges() bool {
	return dm.wrBuf != nil
}

func (dm *DagModifier) Seek(offset int64, whence int) (int64, error) {
	err := dm.Sync()
	if err != nil {
		return 0, err
	}

	fisize, err := dm.Size()
	if err != nil {
		return 0, err
	}

	var newoffset uint64
	switch whence {
	case os.SEEK_CUR:
		newoffset = dm.curWrOff + uint64(offset)
	case os.SEEK_SET:
		newoffset = uint64(offset)
	case os.SEEK_END:
		newoffset = uint64(fisize) - uint64(offset)
	default:
		return 0, ErrUnrecognizedWhence
	}

	if int64(newoffset) > fisize {
		if err := dm.expandSparse(int64(newoffset) - fisize); err != nil {
			return 0, err
		}
	}
	dm.curWrOff = newoffset
	dm.writeStart = newoffset

	if dm.read != nil {
		_, err = dm.read.Seek(offset, whence)
		if err != nil {
			return 0, err
		}
	}

	return int64(dm.curWrOff), nil
}

func (dm *DagModifier) Truncate(size int64) error {
	err := dm.Sync()
	if err != nil {
		return err
	}

	realSize, err := dm.Size()
	if err != nil {
		return err
	}

	// Truncate can also be used to expand the file
	if size > int64(realSize) {
		return dm.expandSparse(int64(size) - realSize)
	}

	nnode, err := dagTruncate(dm.ctx, dm.curNode, uint64(size), dm.dagserv)
	if err != nil {
		return err
	}

	_, err = dm.dagserv.Add(nnode)
	if err != nil {
		return err
	}

	dm.curNode = nnode
	return nil
}

// dagTruncate truncates the given node to 'size' and returns the modified Node
func dagTruncate(ctx context.Context, nd *mdag.ProtoNode, size uint64, ds mdag.DAGService) (*mdag.ProtoNode, error) {
	if len(nd.Links()) == 0 {
		// TODO: this can likely be done without marshaling and remarshaling
		pbn, err := ft.FromBytes(nd.Data())
		if err != nil {
			return nil, err
		}

		nd.SetData(ft.WrapData(pbn.Data[:size]))
		return nd, nil
	}

	var cur uint64
	end := 0
	var modified *mdag.ProtoNode
	ndata := new(ft.FSNode)
	for i, lnk := range nd.Links() {
		child, err := lnk.GetNode(ctx, ds)
		if err != nil {
			return nil, err
		}

		childpb, ok := child.(*mdag.ProtoNode)
		if !ok {
			return nil, err
		}

		childsize, err := ft.DataSize(childpb.Data())
		if err != nil {
			return nil, err
		}

		// found the child we want to cut
		if size < cur+childsize {
			nchild, err := dagTruncate(ctx, childpb, size-cur, ds)
			if err != nil {
				return nil, err
			}

			ndata.AddBlockSize(size - cur)

			modified = nchild
			end = i
			break
		}
		cur += childsize
		ndata.AddBlockSize(childsize)
	}

	_, err := ds.Add(modified)
	if err != nil {
		return nil, err
	}

	nd.SetLinks(nd.Links()[:end])
	err = nd.AddNodeLinkClean("", modified)
	if err != nil {
		return nil, err
	}

	d, err := ndata.GetBytes()
	if err != nil {
		return nil, err
	}

	nd.SetData(d)

	// invalidate cache and recompute serialized data
	_, err = nd.EncodeProtobuf(true)
	if err != nil {
		return nil, err
	}

	return nd, nil
}
