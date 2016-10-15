package filestore

import (
	//"bytes"
	//"encoding/hex"
	"errors"
	"fmt"
	"io"

	dag_pb "github.com/ipfs/go-ipfs/merkledag/pb"
	fs_pb "github.com/ipfs/go-ipfs/unixfs/pb"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
)

type UnixFSInfo struct {
	Type     fs_pb.Data_DataType
	Data     []byte
	FileSize uint64
}

const useFastReconstruct = true

func Reconstruct(data []byte, in io.Reader, blockDataSize uint64) ([]byte, *UnixFSInfo, error) {
	// if blockDataSize == 0 {
	// 	res1, fsinfo1, err1 := reconstruct(data, nil)
	// 	if err1 != nil {
	// 		return res1, fsinfo1, err1
	// 	}
	// 	_ = fsinfo1
	// 	res2, fsinfo2, err2 := reconstructDirect(data, nil, 0)
	// 	_ = fsinfo2
	// 	if err2 != nil {
	// 		panic(err2)
	// 	}
	// 	if !bytes.Equal(res1, res2) {
	// 		println("res1")
	// 		print(hex.Dump(res1))
	// 		println("res2")
	// 		print(hex.Dump(res2))
	// 		panic("Result not equal!")
	// 	}
	// 	return res2, fsinfo2, err2
	// }
	if useFastReconstruct {
		return reconstructDirect(data, in, blockDataSize)
	} else {
		var blockData []byte
		if blockDataSize > 0 {
			blockData = make([]byte, blockDataSize)
			_, err := io.ReadFull(in, blockData)
			if err != nil {
				return nil, nil, err
			}
		}
		return reconstruct(data, blockData)
	}
}

func reconstruct(data []byte, blockData []byte) ([]byte, *UnixFSInfo, error) {
	// Decode data to merkledag protobuffer
	var pbn dag_pb.PBNode
	err := pbn.Unmarshal(data)
	if err != nil {
		panic(err)
	}

	// Decode node's data to unixfs protobuffer
	fs_pbn := new(fs_pb.Data)
	err = proto.Unmarshal(pbn.Data, fs_pbn)
	if err != nil {
		panic(err)
	}

	// gather some data about the unixfs object
	fsinfo := &UnixFSInfo{Type: *fs_pbn.Type, Data: fs_pbn.Data}
	if fs_pbn.Filesize != nil {
		fsinfo.FileSize = *fs_pbn.Filesize
	}

	// if we won't be replasing anything no need to reencode, just
	// return the original data
	if fs_pbn.Data == nil && blockData == nil {
		return data, fsinfo, nil
	}

	fs_pbn.Data = blockData

	// Reencode unixfs protobuffer
	pbn.Data, err = proto.Marshal(fs_pbn)
	if err != nil {
		panic(err)
	}

	// Reencode merkledag protobuffer
	encoded, err := pbn.Marshal()
	if err != nil {
		return nil, fsinfo, err
	}
	return encoded, fsinfo, nil
}

type header struct {
	id int32
	// An "id" of 0 indicates a message we don't care about the
	// value.  As we don't care about the value multiple
	// fields may be concatenated into one.
	wire int32
	// "wire" is the Protocol Buffer wire format
	val uint64
	// The exact meaning of "val" depends on the wire format:
	// if a varint (wire format 0) then val is the value of the
	// variable int; if length-delimited (wire format 2)
	// then val is the payload size; otherwise, val is unused.
}

type field struct {
	header
	offset int
	// "offset" is the offset from the start of the buffer that
	// contains the protocol key-value pair corresponding to the
	// field, the end of the field is the same as the offset of
	// the next field.  An dummy field is added at the end that
	// contains the final offset (i.e. the length of the buffer)
	// to avoid special cases.
}

type fields struct {
	byts []byte
	flds []field
}

func (f fields) data(i int) []byte {
	return f.byts[f.flds[i].offset:f.flds[i+1].offset]
}

func (f fields) size(i int) int {
	return f.flds[i+1].offset - f.flds[i].offset
}

func (f fields) field(i int) field {
	return f.flds[i]
}

func (f fields) fields() []field {
	return f.flds[0 : len(f.flds)-1]
}

// only valid for the length-delimited (2) wire format
func (f fields) payload(i int) []byte {
	return f.byts[f.flds[i+1].offset-int(f.flds[i].val) : f.flds[i+1].offset]
}

const (
	unixfsTypeField     = 1
	unixfsDataField     = 2
	unixfsFilesizeField = 3
)

// An implementation of reconstruct that avoids expensive
// intermertaint data structures and unnecessary copying of data by
// reading the protocol buffer messages directly.
func reconstructDirect(data []byte, blockData io.Reader, blockDataSize uint64) ([]byte, *UnixFSInfo, error) {
	dag, err := decodePB(data, func(typ int32) bool {
		return typ == 1
	})
	var fs fields
	if err != nil {
		return nil, nil, err
	}
	dagSz := 0
	for i, fld := range dag.fields() {
		if fld.id == 1 {
			fs, err = decodePB(dag.payload(i), func(typ int32) bool {
				return typ == unixfsTypeField || typ == unixfsDataField || typ == unixfsFilesizeField
			})
			if err != nil {
				return nil, nil, err
			}
		} else {
			dagSz += dag.size(i)
		}
	}

	fsinfo := new(UnixFSInfo)
	if len(fs.fields()) == 0 {
		return nil, nil, errors.New("no UnixFS data")
	}
	if fs.field(0).id != unixfsTypeField {
		return nil, nil, errors.New("unexpected field order")
	} else {
		fsinfo.Type = fs_pb.Data_DataType(fs.field(0).val)
	}
	fsSz := 0
	for i, fld := range fs.fields() {
		if fld.id == unixfsDataField {
			if i != 1 {
				return nil, nil, errors.New("unexpected field order")
			}
			continue
		}
		if fld.id == unixfsFilesizeField {
			fsinfo.FileSize = fld.val
		}
		fsSz += fs.size(i)
	}
	if len(fs.fields()) >= 2 && fs.field(1).id == unixfsDataField {
		fsinfo.Data = fs.payload(1)
	} else if blockDataSize == 0 {
		// if we won't be replasing anything no need to
		// reencode, just return the original data
		return data, fsinfo, nil
	}
	if blockDataSize > 0 {
		fsSz += 1 /* header */ + sizeVarint(blockDataSize) + int(blockDataSize)
	}
	dagSz += 1 /* header */ + sizeVarint(uint64(fsSz)) + fsSz

	// now reencode

	out := make([]byte, 0, dagSz)

	for i, fld := range dag.fields() {
		if fld.id == 1 {
			out = append(out, dag.data(i)[0])
			out = append(out, proto.EncodeVarint(uint64(fsSz))...)
			out, err = reconstructUnixfs(out, fs, blockData, blockDataSize)
			if err != nil {
				return nil, fsinfo, err
			}
		} else {
			out = append(out, dag.data(i)...)
		}
	}

	if dagSz != len(out) {
		return nil, nil, fmt.Errorf("verification Failed: computed-size(%d) != actual-size(%d)", dagSz, len(out))
	}
	return out, fsinfo, nil
}

func reconstructUnixfs(out []byte, fs fields, blockData io.Reader, blockDataSize uint64) ([]byte, error) {
	// copy first field
	out = append(out, fs.data(0)...)

	// insert Data field
	if blockDataSize > 0 {
		out = append(out, byte((unixfsDataField<<3)|2))
		out = append(out, proto.EncodeVarint(blockDataSize)...)

		origLen := len(out)
		out = out[:origLen+int(blockDataSize)]
		_, err := io.ReadFull(blockData, out[origLen:])
		if err != nil {
			return out, err
		}
	}

	// copy rest of protocol buffer
	sz := len(fs.fields())
	for i := 1; i < sz; i += 1 {
		if fs.field(i).id == unixfsDataField {
			continue
		}
		out = append(out, fs.data(i)...)
	}

	return out, nil
}

func decodePB(data []byte, keep func(int32) bool) (fields, error) {
	res := make([]field, 0, 6)
	offset := 0
	for offset < len(data) {
		hdr, newOffset, err := getField(data, offset)
		if err != nil {
			return fields{}, err
		}
		if !keep(hdr.id) {
			if len(res) > 1 && res[len(res)-1].id == 0 {
				// nothing to do
				// field will get merged into previous field
			} else {
				// set the header id to 0 to indicate
				// we don't care about the value
				res = append(res, field{offset: offset})
			}
		} else {
			res = append(res, field{hdr, offset})
		}
		offset = newOffset
	}
	if offset != len(data) {
		return fields{}, fmt.Errorf("protocol buffer sanity check failed")
	}
	// insert dummy field with the final offset
	res = append(res, field{offset: offset})
	return fields{data, res}, nil
}

func getField(data []byte, offset0 int) (hdr header, offset int, err error) {
	offset = offset0
	hdrVal, varintSz := proto.DecodeVarint(data[offset:])
	if varintSz == 0 {
		err = io.ErrUnexpectedEOF
		return
	}
	offset += varintSz
	hdr.id = int32(hdrVal) >> 3
	hdr.wire = int32(hdrVal) & 0x07
	switch hdr.wire {
	case 0: // Variant
		hdr.val, varintSz = proto.DecodeVarint(data[offset:])
		if varintSz == 0 {
			err = io.ErrUnexpectedEOF
			return
		}
		offset += varintSz
	case 1: // 64 bit
		offset += 8
	case 2: // Length-delimited
		hdr.val, varintSz = proto.DecodeVarint(data[offset:])
		if varintSz == 0 {
			err = io.ErrUnexpectedEOF
			return
		}
		offset += varintSz + int(hdr.val)
	case 5: // 32 bit
		offset += 4
	default:
		err = errors.New("unhandled wire type")
		return
	}
	return
}

// Note: this is copy and pasted from proto/encode.go, newer versions
// have this function exported.  Once upgraded the exported function
// should be used instead.
func sizeVarint(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
