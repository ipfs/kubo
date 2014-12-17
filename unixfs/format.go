// Package format implements a data format for files in the ipfs filesystem
// It is not the only format in ipfs, but it is the one that the filesystem assumes
package unixfs

import (
	"errors"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	pb "github.com/jbenet/go-ipfs/unixfs/pb"
)

var ErrMalformedFileFormat = errors.New("malformed data in file format")
var ErrInvalidDirLocation = errors.New("found directory node in unexpected place")
var ErrUnrecognizedType = errors.New("unrecognized node type")

func FromBytes(data []byte) (*pb.Data, error) {
	pbdata := new(pb.Data)
	err := proto.Unmarshal(data, pbdata)
	if err != nil {
		return nil, err
	}
	return pbdata, nil
}

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

// Returns Bytes that represent a Directory
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

func WrapData(b []byte) []byte {
	pbdata := new(pb.Data)
	typ := pb.Data_Raw
	pbdata.Data = b
	pbdata.Type = &typ

	out, err := proto.Marshal(pbdata)
	if err != nil {
		// This shouldnt happen. seriously.
		panic(err)
	}

	return out
}

func UnwrapData(data []byte) ([]byte, error) {
	pbdata := new(pb.Data)
	err := proto.Unmarshal(data, pbdata)
	if err != nil {
		return nil, err
	}
	return pbdata.GetData(), nil
}

func DataSize(data []byte) (uint64, error) {
	pbdata := new(pb.Data)
	err := proto.Unmarshal(data, pbdata)
	if err != nil {
		return 0, err
	}

	switch pbdata.GetType() {
	case pb.Data_Directory:
		return 0, errors.New("Cant get data size of directory!")
	case pb.Data_File:
		return pbdata.GetFilesize(), nil
	case pb.Data_Raw:
		return uint64(len(pbdata.GetData())), nil
	default:
		return 0, errors.New("Unrecognized node data type!")
	}
}

type MultiBlock struct {
	Data       []byte
	blocksizes []uint64
	subtotal   uint64
}

func (mb *MultiBlock) AddBlockSize(s uint64) {
	mb.subtotal += s
	mb.blocksizes = append(mb.blocksizes, s)
}

func (mb *MultiBlock) GetBytes() ([]byte, error) {
	pbn := new(pb.Data)
	t := pb.Data_File
	pbn.Type = &t
	pbn.Filesize = proto.Uint64(uint64(len(mb.Data)) + mb.subtotal)
	pbn.Blocksizes = mb.blocksizes
	pbn.Data = mb.Data
	return proto.Marshal(pbn)
}

func (mb *MultiBlock) FileSize() uint64 {
	return uint64(len(mb.Data)) + mb.subtotal
}

func (mb *MultiBlock) NumChildren() int {
	return len(mb.blocksizes)
}
