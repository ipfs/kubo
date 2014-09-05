package merkledag

import (
	"bytes"
	"errors"
	"io"

	"code.google.com/p/goprotobuf/proto"
)

var ErrIsDir = errors.New("this dag node is a directory.")

// DagReader provides a way to easily read the data contained in a dag.
type DagReader struct {
	node     *Node
	position int
	buf      *bytes.Buffer
	thisData []byte
}

func NewDagReader(n *Node) (io.Reader, error) {
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
		//TODO: should use dagservice or something to get needed block
		return errors.New("Link to nil node! Tree not fully expanded!")
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
