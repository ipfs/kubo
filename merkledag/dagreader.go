package merkledag

import (
	"bytes"
	"errors"
	"io"

	proto "code.google.com/p/goprotobuf/proto"
	u "github.com/jbenet/go-ipfs/util"
)

var ErrIsDir = errors.New("this dag node is a directory.")

// DagReader provides a way to easily read the data contained in a dag.
type DagReader struct {
	serv     *DAGService
	node     *Node
	position int
	buf      *bytes.Buffer
	thisData []byte
}

func NewDagReader(n *Node, serv *DAGService) (io.Reader, error) {
	pb := new(PBData)
	err := proto.Unmarshal(n.Data, pb)
	if err != nil {
		return nil, err
	}
	switch pb.GetType() {
	case PBData_Directory:
		return nil, ErrIsDir
	case PBData_File:
		return &DagReader{
			node:     n,
			thisData: pb.GetData(),
			serv:     serv,
		}, nil
	case PBData_Raw:
		return bytes.NewBuffer(pb.GetData()), nil
	default:
		panic("Unrecognized node type!")
	}
}

func (dr *DagReader) precalcNextBuf() error {
	if dr.position >= len(dr.node.Links) {
		return io.EOF
	}
	nxtLink := dr.node.Links[dr.position]
	nxt := nxtLink.Node
	if nxt == nil {
		nxtNode, err := dr.serv.Get(u.Key(nxtLink.Hash))
		if err != nil {
			return err
		}
		nxt = nxtNode
	}
	pb := new(PBData)
	err := proto.Unmarshal(nxt.Data, pb)
	if err != nil {
		return err
	}
	dr.position++

	// TODO: dont assume a single layer of indirection
	switch pb.GetType() {
	case PBData_Directory:
		panic("Why is there a directory under a file?")
	case PBData_File:
		//TODO: maybe have a PBData_Block type for indirect blocks?
		panic("Not yet handling different layers of indirection!")
	case PBData_Raw:
		dr.buf = bytes.NewBuffer(pb.GetData())
		return nil
	default:
		panic("Unrecognized node type!")
	}
}

func (dr *DagReader) Read(b []byte) (int, error) {
	if dr.buf == nil {
		err := dr.precalcNextBuf()
		if err != nil {
			return 0, err
		}
	}
	total := 0
	for {
		n, err := dr.buf.Read(b[total:])
		total += n
		if err != nil {
			if err != io.EOF {
				return total, err
			}
		}
		if total == len(b) {
			return total, nil
		}
		err = dr.precalcNextBuf()
		if err != nil {
			return total, err
		}
	}
}

/*
func (dr *DagReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case os.SEEK_SET:
		for i := 0; i < len(dr.node.Links); i++ {
			nsize := dr.node.Links[i].Size - 8
			if offset > nsize {
				offset -= nsize
			} else {
				break
			}
		}
		dr.position = i
		err := dr.precalcNextBuf()
		if err != nil {
			return 0, err
		}
	case os.SEEK_CUR:
	case os.SEEK_END:
	default:
		return 0, errors.New("invalid whence")
	}
	return 0, nil
}
*/
