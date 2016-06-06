package filestore_util

import (
	"fmt"
	"io"
	"os"
	"strings"

	. "github.com/ipfs/go-ipfs/filestore"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/query"
	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	node "github.com/ipfs/go-ipfs/merkledag"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
)

const (
	StatusDefault     = 00 // 00 = default
	StatusOk          = 01 // 0x means no error, but possible problem
	StatusFound       = 02 // 02 = Found key, but not in filestore
	StatusAppended    = 03
	StatusOrphan      = 04
	StatusFileError   = 10 // 1x means error with block
	StatusFileMissing = 11
	StatusFileChanged = 12
	StatusIncomplete  = 20 // 2x means error with non-block node
	StatusError       = 30 // 3x means error with database itself
	StatusKeyNotFound = 31
	StatusCorrupt     = 32
	StatusUnchecked   = 90 // 9x means unchecked
	StatusComplete    = 91
)

func AnInternalError(status int) bool {
	return status == StatusError || status == StatusCorrupt
}

func AnError(status int) bool {
	return 10 <= status && status < 90
}

func OfInterest(status int) bool {
	return status != StatusOk && status != StatusUnchecked && status != StatusComplete
}

func statusStr(status int) string {
	switch status {
	case 0:
		return ""
	case StatusOk:
		return "ok       "
	case StatusFound:
		return "found    "
	case StatusAppended:
		return "appended "
	case StatusOrphan:
		return "orphan   "
	case StatusFileError:
		return "error    "
	case StatusFileMissing:
		return "no-file  "
	case StatusFileChanged:
		return "changed  "
	case StatusIncomplete:
		return "incomplete "
	case StatusError:
		return "ERROR    "
	case StatusKeyNotFound:
		return "missing  "
	case StatusCorrupt:
		return "ERROR    "
	case StatusUnchecked:
		return "         "
	case StatusComplete:
		return "complete "
	default:
		return "??       "
	}
}

type ListRes struct {
	Key ds.Key
	*DataObj
	Status int
}

var EmptyListRes = ListRes{ds.NewKey(""), nil, 0}

func (r *ListRes) What() string {
	if r.WholeFile() {
		return "root"
	} else {
		return "leaf"
	}
}

func (r *ListRes) StatusStr() string {
	str := statusStr(r.Status)
	str = strings.TrimRight(str, " ")
	if str == "" {
		str = "unchecked"
	}
	return str
}

func (r *ListRes) MHash() string {
	return b58.Encode(r.Key.Bytes()[1:])
}

func (r *ListRes) RawHash() []byte {
	return r.Key.Bytes()[1:]
}

func (r *ListRes) Format() string {
	if string(r.RawHash()) == "" {
		return "\n"
	}
	mhash := r.MHash()
	if r.DataObj == nil {
		return fmt.Sprintf("%s%s\n", statusStr(r.Status), mhash)
	} else {
		return fmt.Sprintf("%s%s %s\n", statusStr(r.Status), mhash, r.DataObj.Format())
	}
}

func ListKeys(d *Datastore) (<-chan ListRes, error) {
	qr, err := d.Query(query.Query{KeysOnly: true})
	if err != nil {
		return nil, err
	}

	out := make(chan ListRes, 1024)

	go func() {
		defer close(out)
		for r := range qr.Next() {
			if r.Error != nil {
				return // FIXME
			}
			out <- ListRes{ds.NewKey(r.Key), nil, 0}
		}
	}()
	return out, nil
}

func List(d *Datastore, filter func(ListRes) bool) (<-chan ListRes, error) {
	qr, err := d.Query(query.Query{KeysOnly: true})
	if err != nil {
		return nil, err
	}

	out := make(chan ListRes, 128)

	go func() {
		defer close(out)
		for r := range qr.Next() {
			if r.Error != nil {
				return // FIXME
			}
			key := ds.NewKey(r.Key)
			val, _ := d.GetDirect(key)
			res := ListRes{key, val, 0}
			keep := filter(res)
			if keep {
				out <- res
			}
		}
	}()
	return out, nil
}

func ListAll(d *Datastore) (<-chan ListRes, error) {
	return List(d, func(_ ListRes) bool { return true })
}

func ListWholeFile(d *Datastore) (<-chan ListRes, error) {
	return List(d, func(r ListRes) bool { return r.WholeFile() })
}

func ListByKey(fs *Datastore, keys []k.Key) (<-chan ListRes, error) {
	out := make(chan ListRes, 128)

	go func() {
		defer close(out)
		for _, key := range keys {
			dsKey := key.DsKey()
			dataObj, err := fs.GetDirect(dsKey)
			if err == nil {
				out <- ListRes{dsKey, dataObj, 0}
			}
		}
	}()
	return out, nil
}

func verify(d *Datastore, key ds.Key, val *DataObj, level int) int {
	status := 0
	_, err := d.GetData(key, val, level, true)
	if err == nil {
		status = StatusOk
	} else if os.IsNotExist(err) {
		status = StatusFileMissing
	} else if _, ok := err.(InvalidBlock); ok || err == io.EOF || err == io.ErrUnexpectedEOF {
		status = StatusFileChanged
	} else {
		status = StatusFileError
	}
	return status
}

func fsGetNode(dsKey ds.Key, fs *Datastore) (*node.Node, *DataObj, error) {
	dataObj, err := fs.GetDirect(dsKey)
	if err != nil {
		return nil, nil, err
	}
	if dataObj.NoBlockData() {
		return nil, dataObj, nil
	} else {
		node, err := node.DecodeProtobuf(dataObj.Data)
		if err != nil {
			return nil, nil, err
		}
		return node, dataObj, nil
	}
}

func getNode(dsKey ds.Key, key k.Key, fs *Datastore, bs b.Blockstore) (*node.Node, *DataObj, int) {
	dataObj, err := fs.GetDirect(dsKey)
	if err == nil {
		if dataObj.NoBlockData() {
			return nil, dataObj, StatusUnchecked
		} else {
			node, err := node.DecodeProtobuf(dataObj.Data)
			if err != nil {
				Logger.Errorf("%s: %v", key, err)
				return nil, nil, StatusCorrupt
			}
			return node, dataObj, StatusOk
		}
	}
	block, err2 := bs.Get(key)
	if err == ds.ErrNotFound && err2 == b.ErrNotFound {
		return nil, nil, StatusKeyNotFound
	} else if err2 != nil {
		Logger.Errorf("%s: %v", key, err2)
		return nil, nil, StatusError
	}
	node, err := node.DecodeProtobuf(block.Data())
	if err != nil {
		Logger.Errorf("%s: %v", key, err)
		return nil, nil, StatusCorrupt
	}
	return node, nil, StatusFound
}
