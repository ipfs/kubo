package wasmipld

import (
	"bufio"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-varint"
)

type WacCode uint8

const (
	Null WacCode = 0
	True = 1
	False = 2
	Int = 3
	NInt = 4
	Float = 5
	String = 6
	Bytes = 7
	List = 8
	Map = 9
	Link = 10
)

func WacEncode(n ipld.Node, w io.Writer) error {
	bw, ok := w.(byteAndNormalWriter)
	if !ok {
		bw = bufio.NewWriter(w)
	}

	switch k := n.Kind(); k {
	case ipld.Kind_Null:
		return bw.WriteByte(byte(Null))
	case ipld.Kind_Bool:
		v, err := n.AsBool()
		if err != nil {
			return err
		}
		if v {
			return bw.WriteByte(byte(True))
		}
		return bw.WriteByte(byte(False))
	case ipld.Kind_Int:
		v, err := n.AsInt()
		if err != nil {
			return err
		}
		if v >= 0 {
			if err := bw.WriteByte(byte(Int)); err != nil {
				return err
			}
			_, err = bw.Write(varint.ToUvarint(uint64(v)))
			return err
		} else {
			if err := bw.WriteByte(byte(NInt)); err != nil {
				return err
			}
			_, err = bw.Write(varint.ToUvarint(uint64(-v)))
			return err
		}
	case ipld.Kind_Float:
		return fmt.Errorf("floats unsupported")
	case ipld.Kind_String:
		v, err := n.AsString()
		if err != nil {
			return err
		}

		if err := bw.WriteByte(byte(String)); err != nil {
			return err
		}

		_, err = bw.Write(varint.ToUvarint(uint64(len(v))))
		if err != nil {
			return err
		}

		_, err = bw.Write([]byte(v))
		return err
	case ipld.Kind_Bytes:
		v, err := n.AsString()
		if err != nil {
			return err
		}

		if err := bw.WriteByte(byte(Bytes)); err != nil {
			return err
		}

		_, err = bw.Write(varint.ToUvarint(uint64(len(v))))
		if err != nil {
			return err
		}

		_, err = bw.Write([]byte(v))
		return err
	case ipld.Kind_List:
		if err := bw.WriteByte(byte(List)); err != nil {
			return err
		}

		_, err := bw.Write(varint.ToUvarint(uint64(n.Length())))
		if err != nil {
			return err
		}
		iter := n.ListIterator()
		for !iter.Done() {
			_, v, err := iter.Next()
			if err != nil {
				return err
			}
			if err := WacEncode(v, bw); err != nil {
				return err
			}
		}
		return nil
	case ipld.Kind_Map:
		if err := bw.WriteByte(byte(Map)); err != nil {
			return err
		}

		_, err := bw.Write(varint.ToUvarint(uint64(n.Length())))
		if err != nil {
			return err
		}
		iter := n.MapIterator()
		for !iter.Done() {
			k, v, err := iter.Next()
			if err != nil {
				return err
			}
			if err := WacEncode(k, bw); err != nil {
				return err
			}
			if err := WacEncode(v, bw); err != nil {
				return err
			}
		}
		return nil
	case ipld.Kind_Link:
		if err := bw.WriteByte(byte(Link)); err != nil {
			return err
		}

		v, err := n.AsLink()
		if err != nil {
			return err
		}
		lnk, ok := v.(cidlink.Link)
		if !ok {
			return fmt.Errorf("link was not a cidlink")
		}

		_, err = bw.Write(lnk.Bytes())
		return err
	default:
		return fmt.Errorf("unsupported IPLD kind %s", k)
	}
}

type byteAndNormalWriter interface {
	io.Writer
	io.ByteWriter
}

type byteAndNormalReader interface {
	io.Reader
	io.ByteReader
}

func WacDecode(na ipld.NodeAssembler, r io.Reader) error {
	br, ok := r.(byteAndNormalReader)
	if !ok {
		br = bufio.NewReader(r)
	}

	t, err := br.ReadByte()
	if err != nil{
		return err
	}

	code := WacCode(t)
	switch code {
	case Null:
		return na.AssignNull()
	case True:
		return na.AssignBool(true)
	case False:
		return na.AssignBool(false)
	case Int:
		i, err := varint.ReadUvarint(br)
		if err != nil {
			return err
		}
		return na.AssignInt(int64(i))
	case NInt:
		i, err := varint.ReadUvarint(br)
		if err != nil {
			return err
		}
		return na.AssignInt(-int64(i))
	case Float:
		return fmt.Errorf("floats unsupported")
	case String:
		i, err := varint.ReadUvarint(br)
		if err != nil {
			return err
		}
		buf := make([]byte, i)
		if _, err := io.ReadFull(br, buf); err != nil {
			return err
		}
		return na.AssignString(string(buf))
	case Bytes:
		i, err := varint.ReadUvarint(br)
		if err != nil {
			return err
		}
		buf := make([]byte, i)
		if _, err := io.ReadFull(br, buf); err != nil {
			return err
		}
		return na.AssignBytes(buf)
	case List:
		listElems, err := varint.ReadUvarint(br)
		if err != nil {
			return err
		}
		la, err := na.BeginList(int64(listElems))
		if err != nil {
			return err
		}
		for i := 0; i < int(listElems); i++ {
			if err := WacDecode(la.AssembleValue(), br); err != nil {
				return err
			}
		}
		return la.Finish()
	case Map:
		mapElems, err := varint.ReadUvarint(br)
		if err != nil {
			return err
		}
		ma, err := na.BeginMap(int64(mapElems))
		if err != nil {
			return err
		}
		for i := 0; i < int(mapElems); i++ {
			if err := WacDecode(ma.AssembleKey(), br); err != nil {
				return err
			}
			if err := WacDecode(ma.AssembleValue(), br); err != nil {
				return err
			}
		}
		return ma.Finish()
	case Link:
		_, c, err := cid.CidFromReader(br)
		if err != nil {
			return err
		}
		return na.AssignLink(cidlink.Link{Cid : c})
	default:
		return fmt.Errorf("invalid WAC code")
	}
}

