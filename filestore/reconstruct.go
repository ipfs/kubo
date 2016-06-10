package filestore

import (
	"errors"
	"io"

	dag "github.com/ipfs/go-ipfs/merkledag/pb"
	fs "github.com/ipfs/go-ipfs/unixfs/pb"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
)

type UnixFSInfo struct {
	Type     fs.Data_DataType
	Data     []byte
	FileSize uint64
}

const useFastReconstruct = false

func Reconstruct(data []byte, in io.Reader, blockDataSize uint64) ([]byte, *UnixFSInfo, error) {
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

func reconstruct(data []byte, blockData []byte) ([]byte, *UnixFSInfo, error) {
	// Decode data to merkledag protobuffer
	var pbn dag.PBNode
	err := pbn.Unmarshal(data)
	if err != nil {
		panic(err)
	}

	// Decode node's data to unixfs protobuffer
	fs_pbn := new(fs.Data)
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

type dualBuf struct {
	in  inBuf
	out outBuf
}

type inBuf []byte

type outBuf []byte

type header struct {
	field int
	wire  int
}

// reconstructDirect will reconstruct the block directly without any
// intermediate data structures and without performing any unnecessary
// copies of blockData
func reconstructDirect(data []byte, blockData io.Reader, blockDataSize uint64) ([]byte, error) {
	maxVariantBytes := sizeVarint(uint64(len(data)) + blockDataSize)
	outMaxLen := len(data) + int(blockDataSize) + 1 + maxVariantBytes*2
	buf := dualBuf{data, make([]byte, 0, outMaxLen)}
	for len(buf.in) > 0 {
		hdr, err := buf.getHeader()
		if err != nil {
			return nil, err
		}
		if hdr.field == 1 {
			sz, variantSz := proto.DecodeVarint(buf.in)
			if variantSz == 0 {
				return nil, io.ErrUnexpectedEOF
			}
			buf.in.adv(variantSz)
			if err != nil {
				return nil, err
			}
			unixfsData, err := buf.in.adv(int(sz))
			if err != nil {
				return nil, err
			}
			unixfsSize := uint64(len(unixfsData)) + 1 + uint64(sizeVarint(blockDataSize)) + blockDataSize
			buf.out.append(proto.EncodeVarint(unixfsSize))
			buf.out, err = reconstructUnixfs(unixfsData, buf.out, blockData, blockDataSize)
			if err != nil {
				return nil, err
			}
		} else {
			err = buf.advField(hdr)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(buf.out) > outMaxLen {
		panic("output buffer was too small")
	}

	return buf.out, nil
}

const (
	unixfsTypeField = 1
	unixfsDataField = 2
)

func reconstructUnixfs(data []byte, out outBuf, blockData io.Reader, blockDataSize uint64) (outBuf, error) {
	buf := dualBuf{data, out}
	hdr, err := buf.getHeader()
	if err != nil {
		return buf.out, err
	}
	if hdr.field != unixfsTypeField {
		return buf.out, errors.New("Unexpected field order")
	}
	buf.advField(hdr)

	// insert Data field

	buf.out.append(proto.EncodeVarint((unixfsDataField << 3) | 2))
	buf.out.append(proto.EncodeVarint(blockDataSize))

	origLen := len(buf.out)
	buf.out = buf.out[:origLen+int(blockDataSize)]
	_, err = io.ReadFull(blockData, buf.out[origLen:])
	if err != nil {
		return buf.out, err
	}

	// copy rest of proto buffer

	for len(buf.in) > 0 {
		hdr, err := buf.getHeader()
		if err != nil {
			return buf.out, err
		}
		err = buf.advField(hdr)
		if err != nil {
			return buf.out, err
		}
	}
	return buf.out, err
}

func (b *inBuf) adv(sz int) ([]byte, error) {
	if sz > len(*b) {
		return nil, io.ErrUnexpectedEOF
	}
	data := (*b)[:sz]
	*b = (*b)[sz:]
	return data, nil
}

func (b *outBuf) append(d []byte) {
	*b = append(*b, d...)
}

func (b *dualBuf) adv(sz int) error {
	d, err := b.in.adv(sz)
	if err != nil {
		return err
	}
	b.out.append(d)
	return nil
}

func (b *dualBuf) getVarint() (int, error) {
	val, sz := proto.DecodeVarint(b.in)
	if sz == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	b.adv(sz)
	return int(val), nil
}

func (b *dualBuf) getHeader() (header, error) {
	val, err := b.getVarint()
	if err != nil {
		return header{}, err
	}
	return header{val >> 3, val & 0x07}, nil
}

func (b *dualBuf) advField(hdr header) error {
	switch hdr.wire {
	case 0: // Variant
		_, err := b.getVarint()
		return err
	case 1: // 64 bit
		return b.adv(8)
	case 2: // Length-delimited
		sz, err := b.getVarint()
		if err != nil {
			return err
		}
		return b.adv(sz)
	case 5: // 32 bit
		return b.adv(4)
	default:
		return errors.New("Unhandled wire type")
	}
	return nil
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
