package mod

import (
	"bytes"
	"errors"
	"io"
	"os"
	"time"

	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	mh "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	help "github.com/ipfs/go-ipfs/importer/helpers"
	trickle "github.com/ipfs/go-ipfs/importer/trickle"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	u "github.com/ipfs/go-ipfs/util"
)

var ErrSeekFail = errors.New("failed to seek properly")
var ErrSeekEndNotImpl = errors.New("SEEK_END currently not implemented")
var ErrUnrecognizedWhence = errors.New("unrecognized whence")

// 2MB
var writebufferSize = 1 << 21

var log = u.Logger("dagio")

// DagModifier is the only struct licensed and able to correctly
// perform surgery on a DAG 'file'
// Dear god, please rename this to something more pleasant
type DagModifier struct {
	dagserv mdag.DAGService
	curNode *mdag.Node
	mp      pin.ManualPinner

	splitter   chunk.BlockSplitter
	ctx        context.Context
	readCancel func()

	writeStart uint64
	curWrOff   uint64
	wrBuf      *bytes.Buffer

	read *uio.DagReader
}

func NewDagModifier(ctx context.Context, from *mdag.Node, serv mdag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*DagModifier, error) {
	return &DagModifier{
		curNode:  from.Copy(),
		dagserv:  serv,
		splitter: spl,
		ctx:      ctx,
		mp:       mp,
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
	spl := chunk.SizeSplitter{4096}
	r := io.LimitReader(zeroReader{}, size)
	blks := spl.Split(r)
	nnode, err := dm.appendData(dm.curNode, blks)
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
	pbn, err := ft.FromBytes(dm.curNode.Data)
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

	// Grab key for unpinning after mod operation
	curk, err := dm.curNode.Key()
	if err != nil {
		return err
	}

	// overwrite existing dag nodes
	thisk, done, err := dm.modifyDag(dm.curNode, dm.writeStart, dm.wrBuf)
	if err != nil {
		return err
	}

	nd, err := dm.dagserv.Get(dm.ctx, thisk)
	if err != nil {
		return err
	}

	dm.curNode = nd

	// need to write past end of current dag
	if !done {
		blks := dm.splitter.Split(dm.wrBuf)
		nd, err = dm.appendData(dm.curNode, blks)
		if err != nil {
			return err
		}

		thisk, err = dm.dagserv.Add(nd)
		if err != nil {
			return err
		}

		dm.curNode = nd
	}

	// Finalize correct pinning, and flush pinner
	dm.mp.PinWithMode(thisk, pin.Recursive)
	dm.mp.RemovePinWithMode(curk, pin.Recursive)
	err = dm.mp.Flush()
	if err != nil {
		return err
	}

	dm.writeStart += uint64(buflen)

	dm.wrBuf = nil
	return nil
}

// modifyDag writes the data in 'data' over the data in 'node' starting at 'offset'
// returns the new key of the passed in node and whether or not all the data in the reader
// has been consumed.
func (dm *DagModifier) modifyDag(node *mdag.Node, offset uint64, data io.Reader) (u.Key, bool, error) {
	f, err := ft.FromBytes(node.Data)
	if err != nil {
		return "", false, err
	}

	// If we've reached a leaf node.
	if len(node.Links) == 0 {
		n, err := data.Read(f.Data[offset:])
		if err != nil && err != io.EOF {
			return "", false, err
		}

		// Update newly written node..
		b, err := proto.Marshal(f)
		if err != nil {
			return "", false, err
		}

		nd := &mdag.Node{Data: b}
		k, err := dm.dagserv.Add(nd)
		if err != nil {
			return "", false, err
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
			// Unpin block
			ckey := u.Key(node.Links[i].Hash)
			dm.mp.RemovePinWithMode(ckey, pin.Indirect)

			child, err := node.Links[i].GetNode(dm.ctx, dm.dagserv)
			if err != nil {
				return "", false, err
			}
			k, sdone, err := dm.modifyDag(child, offset-cur, data)
			if err != nil {
				return "", false, err
			}

			// pin the new node
			dm.mp.PinWithMode(k, pin.Indirect)

			offset += bs
			node.Links[i].Hash = mh.Multihash(k)

			// Recache serialized node
			_, err = node.Encoded(true)
			if err != nil {
				return "", false, err
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
func (dm *DagModifier) appendData(node *mdag.Node, blks <-chan []byte) (*mdag.Node, error) {
	dbp := &help.DagBuilderParams{
		Dagserv:  dm.dagserv,
		Maxlinks: help.DefaultLinksPerBlock,
		Pinner:   dm.mp,
	}

	return trickle.TrickleAppend(node, dbp.New(blks))
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
			return err
		}

		i, err := dr.Seek(int64(dm.curWrOff), os.SEEK_SET)
		if err != nil {
			return err
		}

		if i != int64(dm.curWrOff) {
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
func (dm *DagModifier) GetNode() (*mdag.Node, error) {
	err := dm.Sync()
	if err != nil {
		return nil, err
	}
	return dm.curNode.Copy(), nil
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

	switch whence {
	case os.SEEK_CUR:
		dm.curWrOff += uint64(offset)
		dm.writeStart = dm.curWrOff
	case os.SEEK_SET:
		dm.curWrOff = uint64(offset)
		dm.writeStart = uint64(offset)
	case os.SEEK_END:
		return 0, ErrSeekEndNotImpl
	default:
		return 0, ErrUnrecognizedWhence
	}

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

	nnode, err := dagTruncate(dm.curNode, uint64(size), dm.dagserv)
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
func dagTruncate(nd *mdag.Node, size uint64, ds mdag.DAGService) (*mdag.Node, error) {
	if len(nd.Links) == 0 {
		// TODO: this can likely be done without marshaling and remarshaling
		pbn, err := ft.FromBytes(nd.Data)
		if err != nil {
			return nil, err
		}

		nd.Data = ft.WrapData(pbn.Data[:size])
		return nd, nil
	}

	var cur uint64
	end := 0
	var modified *mdag.Node
	ndata := new(ft.FSNode)
	for i, lnk := range nd.Links {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
		defer cancel()

		child, err := lnk.GetNode(ctx, ds)
		if err != nil {
			return nil, err
		}

		childsize, err := ft.DataSize(child.Data)
		if err != nil {
			return nil, err
		}

		// found the child we want to cut
		if size < cur+childsize {
			nchild, err := dagTruncate(child, size-cur, ds)
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

	nd.Links = nd.Links[:end]
	err = nd.AddNodeLinkClean("", modified)
	if err != nil {
		return nil, err
	}

	d, err := ndata.GetBytes()
	if err != nil {
		return nil, err
	}

	nd.Data = d

	// invalidate cache and recompute serialized data
	_, err = nd.Encoded(true)
	if err != nil {
		return nil, err
	}

	return nd, nil
}
