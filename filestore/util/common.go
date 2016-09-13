package filestore_util

import (
	"fmt"
	"io"
	"os"
	"strings"

	. "github.com/ipfs/go-ipfs/filestore"
	. "github.com/ipfs/go-ipfs/filestore/support"

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
	//ShowOrphans = 1
	ShowSpecified = 2
	ShowTopLevel = 3
	//ShowFirstProblem = unimplemented
	ShowProblemChildren = 5
	ShowChildren = 7
)

const (
	StatusDefault     =  0 // 00 = default
	StatusOk          =  1 // 01 = leaf node okay
	StatusAllPartsOk  =  2 // 02 = all children have "ok" status
	StatusFound       =  5 // 05 = Found key, but not in filestore
	StatusOrphan      =  8
	StatusAppended    =  9
	StatusFileError   = 10 // 1x means error with block
	StatusFileMissing = 11
	StatusFileChanged = 12
	StatusIncomplete  = 20 // 2x means error with non-block node
	StatusProblem     = 21 // 21 if some children exist but could not be read
	StatusError       = 30 // 3x means error with database itself
	StatusKeyNotFound = 31
	StatusCorrupt     = 32
	StatusUnchecked   = 80 // 8x means unchecked
	StatusComplete    = 82 // 82 = All parts found
	StatusMarked      = 90 // 9x is for internal use
)

func AnInternalError(status int) bool {
	return status == StatusError || status == StatusCorrupt
}

func AnError(status int) bool {
	return 10 <= status && status < 80
}

func IsOk(status int) bool {
	return status == StatusOk || status == StatusAllPartsOk
}

func Unchecked(status int) bool {
	return status == StatusUnchecked || status == StatusComplete
}

func InternalNode(status int) bool {
	return status == StatusAllPartsOk || status == StatusIncomplete ||
		status == StatusProblem || status == StatusComplete 
}

func OfInterest(status int) bool {
	return !IsOk(status) && !Unchecked(status)
}

func statusStr(status int) string {
	switch status {
	case 0:
		return ""
	case StatusOk, StatusAllPartsOk:
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
	case StatusProblem:
		return "problem  "
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
		return fmt.Sprintf("?%02d      ", status)
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

func MHash(dsKey ds.Key) string {
	key, err := k.KeyFromDsKey(dsKey)
	if err != nil {
		return "??????????????????????????????????????????????"
	}
	return key.B58String()
}

func (r *ListRes) MHash() string {
	return MHash(r.Key)
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

func ListKeys(d *Basic) <-chan ListRes {
	ch, _ := List(d, nil, true)
	return ch
}

type ListFilter func(*DataObj) bool

func List(d *Basic, filter ListFilter, keysOnly bool) (<-chan ListRes, error) {
	iter := ListIterator{d.NewIterator(), filter}

	if keysOnly {
		out := make(chan ListRes, 1024)
		go func() {
			defer close(out)
			for iter.Next() {
				out <- ListRes{Key: iter.Key()}
			}
		}()
		return out, nil
	} else {
		out := make(chan ListRes, 128)
		go func() {
			defer close(out)
			for iter.Next() {
				res := ListRes{Key: iter.Key()}
				_, res.DataObj, _ = iter.Value()
				out <- res
			}
		}()
		return out, nil
	}
}

var ListFilterAll ListFilter = nil

func ListFilterWholeFile(r *DataObj) bool {return r.WholeFile()}

func ListByKey(fs *Basic, keys []k.Key) (<-chan ListRes, error) {
	out := make(chan ListRes, 128)

	go func() {
		defer close(out)
		for _, key := range keys {
			dsKey := key.DsKey()
			_, dataObj, err := fs.GetDirect(dsKey)
			if err == nil {
				out <- ListRes{dsKey, dataObj, 0}
			}
		}
	}()
	return out, nil
}

type ListIterator struct {
	*Iterator
	Filter ListFilter
}

func (itr ListIterator) Next() bool {
	for itr.Iterator.Next() {
		if itr.Filter == nil {
			return true
		}
		_, val, _ := itr.Value()
		if val == nil {
			// an error ...
			return true
		}
		keep := itr.Filter(val)
		if keep {
			return true
		}
		// else continue to next value
	}
	return false
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

func getNode(dsKey ds.Key, fs *Basic, bs b.Blockstore) ([]byte, *DataObj, []*node.Link, int) {
	origData, dataObj, err := fs.GetDirect(dsKey)
	if err == nil {
		if dataObj.NoBlockData() {
			return origData, dataObj, nil, StatusUnchecked
		} else {
			links, err := GetLinks(dataObj)
			if err != nil {
				Logger.Errorf("%s: %v", MHash(dsKey), err)
				return origData, nil, nil, StatusCorrupt
			}
			return origData, dataObj, links, StatusOk
		}
	}
	key, err2 := k.KeyFromDsKey(dsKey)
	if err2 != nil {
		Logger.Errorf("%s: %v", key, err2)
		return nil, nil, nil, StatusError
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
	return nil, nil, node.Links, StatusFound
}
