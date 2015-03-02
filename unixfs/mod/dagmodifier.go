package mod

import (
	"bytes"
	"errors"
	"io"
	"os"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	help "github.com/jbenet/go-ipfs/importer/helpers"
	trickle "github.com/jbenet/go-ipfs/importer/trickle"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	pin "github.com/jbenet/go-ipfs/pin"
	ft "github.com/jbenet/go-ipfs/unixfs"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	ftpb "github.com/jbenet/go-ipfs/unixfs/pb"
	u "github.com/jbenet/go-ipfs/util"
)

var writebufferSize = 1 << 20

var log = u.Logger("dagio")

// DagModifier is the only struct licensed and able to correctly
// perform surgery on a DAG 'file'
// Dear god, please rename this to something more pleasant
type DagModifier struct {
	dagserv mdag.DAGService
	curNode *mdag.Node
	mp      pin.ManualPinner

	pbdata   *ftpb.Data
	splitter chunk.BlockSplitter
	ctx      context.Context

	writeStart uint64
	curWrOff   uint64
	wrBuf      *bytes.Buffer

	read *uio.DagReader
}

func NewDagModifier(ctx context.Context, from *mdag.Node, serv mdag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*DagModifier, error) {
	pbd, err := ft.FromBytes(from.Data)
	if err != nil {
		return nil, err
	}

	return &DagModifier{
		curNode:  from.Copy(),
		dagserv:  serv,
		pbdata:   pbd,
		splitter: spl,
		ctx:      ctx,
		mp:       mp,
	}, nil
}

// WriteAt will modify a dag file in place
// NOTE: it currently assumes only a single level of indirection
func (dm *DagModifier) WriteAt(b []byte, offset uint64) (int, error) {
	// TODO: this is currently VERY inneficient
	if offset != dm.curWrOff {
		err := dm.Flush()
		if err != nil {
			return 0, err
		}
		dm.writeStart = offset
	}

	return dm.Write(b)
}

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
		err := dm.Flush()
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (dm *DagModifier) Size() uint64 {
	if dm == nil {
		return 0
	}
	return dm.pbdata.GetFilesize()
}

func (dm *DagModifier) Flush() error {
	if dm.wrBuf == nil {
		return nil
	}
	k, _, done, err := dm.modifyDag(dm.curNode, dm.writeStart, dm.wrBuf)
	if err != nil {
		return err
	}

	nd, err := dm.dagserv.Get(k)
	if err != nil {
		return err
	}

	dm.curNode = nd

	if !done {
		blks := dm.splitter.Split(dm.wrBuf)
		nd, err = dm.appendData(dm.curNode, blks)
		if err != nil {
			return err
		}

		_, err := dm.dagserv.Add(nd)
		if err != nil {
			return err
		}

		dm.curNode = nd
	}

	dm.wrBuf = nil
	return nil
}

func (dm *DagModifier) modifyDag(node *mdag.Node, offset uint64, data io.Reader) (u.Key, int, bool, error) {
	f, err := ft.FromBytes(node.Data)
	if err != nil {
		return "", 0, false, err
	}

	if len(node.Links) == 0 && (f.GetType() == ftpb.Data_Raw || f.GetType() == ftpb.Data_File) {
		// A data block! lets write to it

		n, err := data.Read(f.Data[offset:])
		if err != nil && err != io.EOF {
			return "", 0, false, err
		}

		// Update newly written node..
		b, err := proto.Marshal(f)
		if err != nil {
			return "", 0, false, err
		}

		nd := &mdag.Node{Data: b}
		k, err := dm.dagserv.Add(nd)
		if err != nil {
			return "", 0, false, err
		}

		// Hey look! we're done!
		var done bool
		if n < len(f.Data) {
			done = true
		}

		return k, n, done, nil
	}

	var cur uint64
	var done bool
	var totread int
	for i, bs := range f.GetBlocksizes() {
		if cur+bs > offset {
			child, err := node.Links[i].GetNode(dm.dagserv)
			if err != nil {
				return "", 0, false, err
			}
			k, nread, sdone, err := dm.modifyDag(child, offset-cur, data)
			if err != nil {
				return "", 0, false, err
			}
			totread += nread

			offset += bs
			node.Links[i].Hash = mh.Multihash(k)

			if sdone {
				done = true
				break
			}
		}
		cur += bs
	}

	k, err := dm.dagserv.Add(node)
	return k, totread, done, err
}

func (dm *DagModifier) appendData(node *mdag.Node, blks <-chan []byte) (*mdag.Node, error) {
	dbp := &help.DagBuilderParams{
		Dagserv:  dm.dagserv,
		Maxlinks: 2, //help.DefaultLinksPerBlock,
		Pinner:   dm.mp,
	}

	return trickle.TrickleAppend(node, dbp.New(blks))
}

func (dm *DagModifier) Read(b []byte) (int, error) {
	err := dm.Flush()
	if err != nil {
		return 0, err
	}

	if dm.read == nil {
		dr, err := uio.NewDagReader(dm.ctx, dm.curNode, dm.dagserv)
		if err != nil {
			return 0, err
		}

		i, err := dr.Seek(int64(dm.curWrOff), os.SEEK_SET)
		if err != nil {
			return 0, err
		}

		if i != int64(dm.curWrOff) {
			return 0, errors.New("failed to seek properly")
		}

		dm.read = dr
	}

	n, err := dm.read.Read(b)
	dm.curWrOff += uint64(n)
	return n, err
}

// splitBytes uses a splitterFunc to turn a large array of bytes
// into many smaller arrays of bytes
func (dm *DagModifier) splitBytes(in io.Reader) ([]u.Key, error) {
	var out []u.Key
	blks := dm.splitter.Split(in)
	for blk := range blks {
		nd := help.NewUnixfsNode()
		nd.SetData(blk)
		dagnd, err := nd.GetDagNode()
		if err != nil {
			return nil, err
		}

		k, err := dm.dagserv.Add(dagnd)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

// GetNode gets the modified DAG Node
func (dm *DagModifier) GetNode() (*mdag.Node, error) {
	err := dm.Flush()
	if err != nil {
		return nil, err
	}
	return dm.curNode.Copy(), nil
}
