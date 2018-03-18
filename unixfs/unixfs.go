// Package unixfs implements a data format for files in the IPFS filesystem It
// is not the only format in ipfs, but it is the one that the filesystem
// assumes
package unixfs

import (
	"errors"

	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"

	dag "github.com/ipfs/go-ipfs/merkledag"
	pb "github.com/ipfs/go-ipfs/unixfs/pb"
)

// Shorthands for protobuffer types
const (
	TRaw       = pb.Data_Raw
	TFile      = pb.Data_File
	TDirectory = pb.Data_Directory
	TMetadata  = pb.Data_Metadata
	TSymlink   = pb.Data_Symlink
	THAMTShard = pb.Data_HAMTShard
)

// Common errors
var (
	ErrMalformedFileFormat = errors.New("malformed data in file format")
	ErrInvalidDirLocation  = errors.New("found directory node in unexpected place")
	ErrUnrecognizedType    = errors.New("unrecognized node type")
)

// FromBytes unmarshals a byte slice as protobuf Data.
func FromBytes(data []byte) (*pb.Data, error) {
	pbdata := new(pb.Data)
	err := proto.Unmarshal(data, pbdata)
	if err != nil {
		return nil, err
	}
	return pbdata, nil
}

// FilePBData creates a protobuf File with the given
// byte slice and returns the marshaled protobuf bytes representing it.
func FilePBData(data []byte, totalsize uint64) []byte {
	pbfile := new(pb.Data)
	typ := pb.Data_File
	pbfile.Type = &typ
	pbfile.Data = data
	pbfile.Filesize = proto.Uint64(totalsize)

	data, err := proto.Marshal(pbfile)
	if err != nil {
		// This really shouldnt happen, i promise
		// The only failure case for marshal is if required fields
		// are not filled out, and they all are. If the proto object
		// gets changed and nobody updates this function, the code
		// should panic due to programmer error
		panic(err)
	}
	return data
}

//FolderPBData returns Bytes that represent a Directory.
func FolderPBData() []byte {
	pbfile := new(pb.Data)
	typ := pb.Data_Directory
	pbfile.Type = &typ

	data, err := proto.Marshal(pbfile)
	if err != nil {
		//this really shouldnt happen, i promise
		panic(err)
	}
	return data
}

//WrapData marshals raw bytes into a `Data_Raw` type protobuf message.
func WrapData(b []byte) []byte {
	pbdata := new(pb.Data)
	typ := pb.Data_Raw
	pbdata.Data = b
	pbdata.Type = &typ
	pbdata.Filesize = proto.Uint64(uint64(len(b)))

	out, err := proto.Marshal(pbdata)
	if err != nil {
		// This shouldnt happen. seriously.
		panic(err)
	}

	return out
}

//SymlinkData returns a `Data_Symlink` protobuf message for the path you specify.
func SymlinkData(path string) ([]byte, error) {
	pbdata := new(pb.Data)
	typ := pb.Data_Symlink
	pbdata.Data = []byte(path)
	pbdata.Type = &typ

	out, err := proto.Marshal(pbdata)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// UnwrapData unmarshals a protobuf messages and returns the contents.
func UnwrapData(data []byte) ([]byte, error) {
	pbdata := new(pb.Data)
	err := proto.Unmarshal(data, pbdata)
	if err != nil {
		return nil, err
	}
	return pbdata.GetData(), nil
}

// DataSize returns the size of the contents in protobuf wrapped slice.
// For raw data it simply provides the length of it. For Data_Files, it
// will return the associated filesize. Note that Data_Directories will
// return an error.
func DataSize(data []byte) (uint64, error) {
	pbdata := new(pb.Data)
	err := proto.Unmarshal(data, pbdata)
	if err != nil {
		return 0, err
	}

	switch pbdata.GetType() {
	case pb.Data_Directory:
		return 0, errors.New("can't get data size of directory")
	case pb.Data_File:
		return pbdata.GetFilesize(), nil
	case pb.Data_Raw:
		return uint64(len(pbdata.GetData())), nil
	default:
		return 0, errors.New("unrecognized node data type")
	}
}

// An FSNode represents a filesystem object.
type FSNode struct {
	Data []byte

	// total data size for each child
	blocksizes []uint64

	// running sum of blocksizes
	subtotal uint64

	// node type of this node
	Type pb.Data_DataType
}

// FSNodeFromBytes unmarshal a protobuf message onto an FSNode.
func FSNodeFromBytes(b []byte) (*FSNode, error) {
	pbn := new(pb.Data)
	err := proto.Unmarshal(b, pbn)
	if err != nil {
		return nil, err
	}

	n := new(FSNode)
	n.Data = pbn.Data
	n.blocksizes = pbn.Blocksizes
	n.subtotal = pbn.GetFilesize() - uint64(len(n.Data))
	n.Type = pbn.GetType()
	return n, nil
}

// AddBlockSize adds the size of the next child block of this node
func (n *FSNode) AddBlockSize(s uint64) {
	n.subtotal += s
	n.blocksizes = append(n.blocksizes, s)
}

// RemoveBlockSize removes the given child block's size.
func (n *FSNode) RemoveBlockSize(i int) {
	n.subtotal -= n.blocksizes[i]
	n.blocksizes = append(n.blocksizes[:i], n.blocksizes[i+1:]...)
}

// GetBytes marshals this node as a protobuf message.
func (n *FSNode) GetBytes() ([]byte, error) {
	pbn := new(pb.Data)
	pbn.Type = &n.Type
	pbn.Filesize = proto.Uint64(uint64(len(n.Data)) + n.subtotal)
	pbn.Blocksizes = n.blocksizes
	pbn.Data = n.Data
	return proto.Marshal(pbn)
}

// FileSize returns the total size of this tree. That is, the size of
// the data in this node plus the size of all its children.
func (n *FSNode) FileSize() uint64 {
	return uint64(len(n.Data)) + n.subtotal
}

// NumChildren returns the number of child blocks of this node
func (n *FSNode) NumChildren() int {
	return len(n.blocksizes)
}

// Metadata is used to store additional FSNode information.
type Metadata struct {
	MimeType string
	Size     uint64
}

// MetadataFromBytes Unmarshals a protobuf Data message into Metadata.
// The provided slice should have been encoded with BytesForMetadata().
func MetadataFromBytes(b []byte) (*Metadata, error) {
	pbd := new(pb.Data)
	err := proto.Unmarshal(b, pbd)
	if err != nil {
		return nil, err
	}
	if pbd.GetType() != pb.Data_Metadata {
		return nil, errors.New("incorrect node type")
	}

	pbm := new(pb.Metadata)
	err = proto.Unmarshal(pbd.Data, pbm)
	if err != nil {
		return nil, err
	}
	md := new(Metadata)
	md.MimeType = pbm.GetMimeType()
	return md, nil
}

// Bytes marshals Metadata as a protobuf message of Metadata type.
func (m *Metadata) Bytes() ([]byte, error) {
	pbm := new(pb.Metadata)
	pbm.MimeType = &m.MimeType
	return proto.Marshal(pbm)
}

// BytesForMetadata wraps the given Metadata as a profobuf message of Data type,
// setting the DataType to Metadata. The wrapped bytes are itself the
// result of calling m.Bytes().
func BytesForMetadata(m *Metadata) ([]byte, error) {
	pbd := new(pb.Data)
	pbd.Filesize = proto.Uint64(m.Size)
	typ := pb.Data_Metadata
	pbd.Type = &typ
	mdd, err := m.Bytes()
	if err != nil {
		return nil, err
	}

	pbd.Data = mdd
	return proto.Marshal(pbd)
}

// EmptyDirNode creates an empty folder Protonode.
func EmptyDirNode() *dag.ProtoNode {
	return dag.NodeWithData(FolderPBData())
}
