package filestore_util

import (
	"fmt"
	"io"
	"os"
	"strings"

	. "github.com/ipfs/go-ipfs/filestore"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	node "github.com/ipfs/go-ipfs/merkledag"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	//"gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/query"
)

type VerifyLevel int

const (
	CheckExists VerifyLevel = iota
	CheckFast
	CheckIfChanged
	CheckAlways
)

func VerifyLevelFromNum(level int) (VerifyLevel, error) {
	switch level {
	case 0, 1:
		return CheckExists, nil
	case 2, 3:
		return CheckFast, nil
	case 4, 5, 6:
		return CheckIfChanged, nil
	case 7, 8, 9:
		return CheckAlways, nil
	default:
		return -1, fmt.Errorf("verify level must be between 0-9: %d", level)
	}
}

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
	OrigData []byte
	*DataObj
	Status int
}

var EmptyListRes = ListRes{ds.NewKey(""), nil, nil, 0}

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
	key, err := k.KeyFromDsKey(r.Key)
	if err != nil {
		return "??????????????????????????????????????????????"
	}
	return key.B58String()
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

func ListKeys(d *Basic) (<-chan ListRes, error) {
	iter := d.DB().NewIterator(nil, nil)

	out := make(chan ListRes, 1024)

	go func() {
		defer close(out)
		for iter.Next() {
			out <- ListRes{ds.NewKey(string(iter.Key())), nil, nil, 0}
		}
	}()
	return out, nil
}

func List(d *Basic, filter func(ListRes) bool) (<-chan ListRes, error) {
	iter := d.DB().NewIterator(nil, nil)

	out := make(chan ListRes, 128)

	go func() {
		defer close(out)
		for iter.Next() {
			key := ds.NewKey(string(iter.Key()))
			_, val, _ := Decode(iter.Value())
			res := ListRes{key, iter.Value(), val, 0}
			keep := filter(res)
			if keep {
				out <- res
			}
		}
	}()
	return out, nil
}

func ListAll(d *Basic) (<-chan ListRes, error) {
	return List(d, func(_ ListRes) bool { return true })
}

func ListWholeFile(d *Basic) (<-chan ListRes, error) {
	return List(d, func(r ListRes) bool { return r.WholeFile() })
}

func ListByKey(fs *Basic, keys []k.Key) (<-chan ListRes, error) {
	out := make(chan ListRes, 128)

	go func() {
		defer close(out)
		for _, key := range keys {
			dsKey := key.DsKey()
			origData, dataObj, err := fs.GetDirect(dsKey)
			if err == nil {
				out <- ListRes{dsKey, origData, dataObj, 0}
			}
		}
	}()
	return out, nil
}

func verify(d *Basic, key ds.Key, origData []byte, val *DataObj, level VerifyLevel) int {
	var err error
	switch level {
	case CheckExists:
		return StatusUnchecked
	case CheckFast:
		err = VerifyFast(key, val)
	case CheckIfChanged:
		_, err = GetData(d.AsFull(), key, origData, val, VerifyIfChanged)
	case CheckAlways:
		_, err = GetData(d.AsFull(), key, origData, val, VerifyAlways)
	default:
		return StatusError
	}

	if err == nil {
		return StatusOk
	} else if os.IsNotExist(err) {
		return StatusFileMissing
	} else if _, ok := err.(InvalidBlock); ok || err == io.EOF || err == io.ErrUnexpectedEOF {
		return StatusFileChanged
	} else {
		return StatusFileError
	}
}

func fsGetNode(dsKey ds.Key, fs *Datastore) (*node.Node, *DataObj, error) {
	_, dataObj, err := fs.GetDirect(dsKey)
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

func getNode(dsKey ds.Key, key k.Key, fs *Basic, bs b.Blockstore) (*node.Node, []byte, *DataObj, int) {
	origData, dataObj, err := fs.GetDirect(dsKey)
	if err == nil {
		if dataObj.NoBlockData() {
			return nil, origData, dataObj, StatusUnchecked
		} else {
			node, err := node.DecodeProtobuf(dataObj.Data)
			if err != nil {
				Logger.Errorf("%s: %v", key, err)
				return nil, origData, nil, StatusCorrupt
			}
			return node, origData, dataObj, StatusOk
		}
	}
	block, err2 := bs.Get(key)
	if err == ds.ErrNotFound && err2 == b.ErrNotFound {
		return nil, nil, nil, StatusKeyNotFound
	} else if err2 != nil {
		Logger.Errorf("%s: %v", key, err2)
		return nil, nil, nil, StatusError
	}
	node, err := node.DecodeProtobuf(block.Data())
	if err != nil {
		Logger.Errorf("%s: %v", key, err)
		return nil, nil, nil, StatusCorrupt
	}
	return node, nil, nil, StatusFound
}
