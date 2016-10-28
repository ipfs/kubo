package filestore_util

import (
	"fmt"
	"io"
	"os"
	"strings"

	. "github.com/ipfs/go-ipfs/filestore"
	. "github.com/ipfs/go-ipfs/filestore/support"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	dag "github.com/ipfs/go-ipfs/merkledag"
	node "gx/ipfs/QmU7bFWQ793qmvNy7outdCaMfSDNk8uqhx4VNrxYj5fj5g/go-ipld-node"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	//cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	//"gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/query"
	//dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
)

type VerifyLevel int

const (
	CheckExists VerifyLevel = iota
	CheckFast
	CheckIfChanged
	CheckAlways
)

func VerifyLevelFromNum(fs *Basic, level int) (VerifyLevel, error) {
	switch level {
	case 0, 1:
		return CheckExists, nil
	case 2, 3:
		return CheckFast, nil
	case 4, 5:
		return CheckIfChanged, nil
	case 6:
		if fs.Verify() <= VerifyIfChanged {
			return CheckIfChanged, nil
		} else {
			return CheckAlways, nil
		}
	case 7, 8, 9:
		return CheckAlways, nil
	default:
		return -1, fmt.Errorf("verify level must be between 0-9: %d", level)
	}
}

const (
	//ShowOrphans = 1
	ShowSpecified = 2
	ShowTopLevel  = 3
	//ShowFirstProblem = unimplemented
	ShowProblemChildren = 5
	ShowChildren        = 7
)

type Status int16

const (
	StatusNone Status = 0 // 00 = default

	CategoryOk       Status = 0
	StatusOk         Status = 1 // 01 = leaf node okay
	StatusAllPartsOk Status = 2 // 02 = all children have "ok" status
	StatusFound      Status = 5 // 05 = Found key, but not in filestore
	StatusOrphan     Status = 8
	StatusAppended   Status = 9

	CategoryBlockErr  Status = 10 // 1x means error with block
	StatusFileError   Status = 10
	StatusFileMissing Status = 11
	StatusFileChanged Status = 12
	StatusFileTouched Status = 13

	CategoryNodeErr  Status = 20 // 2x means error with non-block node
	StatusProblem    Status = 20 // 20 if some children exist but could not be read
	StatusIncomplete Status = 21

	CategoryOtherErr  Status = 30 // 3x means error with database itself
	StatusError       Status = 30
	StatusCorrupt     Status = 31
	StatusKeyNotFound Status = 32

	CategoryUnchecked Status = 80 // 8x means unchecked
	StatusUnchecked   Status = 80
	StatusComplete    Status = 82 // 82 = All parts found

	CategoryInternal Status = 90
	StatusMarked     Status = 90 // 9x is for internal use
)

func AnInternalError(status Status) bool {
	return status == StatusError || status == StatusCorrupt
}

func AnError(status Status) bool {
	return Status(10) <= status && status < Status(80)
}

func IsOk(status Status) bool {
	return status == StatusOk || status == StatusAllPartsOk
}

func Unchecked(status Status) bool {
	return status == StatusUnchecked || status == StatusComplete
}

func InternalNode(status Status) bool {
	return status == StatusAllPartsOk || status == StatusIncomplete ||
		status == StatusProblem || status == StatusComplete
}

func OfInterest(status Status) bool {
	return !IsOk(status) && !Unchecked(status)
}

func statusStr(status Status) string {
	switch status {
	case StatusNone:
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
	case StatusFileTouched:
		return "touched  "
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
	Key Key
	*DataObj
	Status Status
}

var EmptyListRes = ListRes{Key{"", "", -1}, nil, 0}

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
	return MHash(r.Key)
}

func (r *ListRes) FormatHashOnly() string {
	if r.Key.Hash == "" {
		return "\n"
	} else {
		return fmt.Sprintf("%s%s\n", statusStr(r.Status), MHash(r.Key))
	}
}

func (r *ListRes) FormatKeyOnly() string {
	if r.Key.Hash == "" {
		return "\n"
	} else {
		return fmt.Sprintf("%s%s\n", statusStr(r.Status), r.Key.Format())
	}
}

func (r *ListRes) FormatDefault() string {
	if r.Key.Hash == "" {
		return "\n"
	} else if r.DataObj == nil {
		return fmt.Sprintf("%s%s\n", statusStr(r.Status), r.Key.Format())
	} else {
		return fmt.Sprintf("%s%s\n", statusStr(r.Status), r.DataObj.KeyStr(r.Key))
	}
}

func (r *ListRes) FormatWithType() string {
	if r.Key.Hash == "" {
		return "\n"
	} else if r.DataObj == nil {
		return fmt.Sprintf("%s      %s\n", statusStr(r.Status), r.Key.Format())
	} else {
		return fmt.Sprintf("%s%-5s %s\n", statusStr(r.Status), r.TypeStr(), r.DataObj.KeyStr(r.Key))
	}
}

func (r *ListRes) FormatLong() string {
	if r.Key.Hash == "" {
		return "\n"
	} else if r.DataObj == nil {
		return fmt.Sprintf("%s%49s  %s\n", statusStr(r.Status), "", r.Key.Format())
	} else if r.NoBlockData() {
		return fmt.Sprintf("%s%-5s %12d %30s  %s\n", statusStr(r.Status), r.TypeStr(), r.Size, r.DateStr(), r.DataObj.KeyStr(r.Key))
	} else {
		return fmt.Sprintf("%s%-5s %12d %30s  %s\n", statusStr(r.Status), r.TypeStr(), r.Size, "", r.DataObj.KeyStr(r.Key))
	}
}

func StrToFormatFun(str string) (func(*ListRes) string, error) {
	switch str {
	case "hash":
		return (*ListRes).FormatHashOnly, nil
	case "key":
		return (*ListRes).FormatKeyOnly, nil
	case "default", "":
		return (*ListRes).FormatDefault, nil
	case "w/type":
		return (*ListRes).FormatWithType, nil
	case "long":
		return (*ListRes).FormatLong, nil
	default:
		return nil, fmt.Errorf("invalid format type: %s", str)
	}
}

func ListKeys(d *Basic) <-chan ListRes {
	ch, _ := List(d, nil, true)
	return ch
}

type ListFilter func(*DataObj) bool

func List(d *Basic, filter ListFilter, keysOnly bool) (<-chan ListRes, error) {
	iter := ListIterator{d.DB().NewIterator(), filter}

	if keysOnly {
		out := make(chan ListRes, 1024)
		go func() {
			defer close(out)
			for iter.Next() {
				out <- ListRes{Key: iter.Key().Key}
			}
		}()
		return out, nil
	} else {
		out := make(chan ListRes, 128)
		go func() {
			defer close(out)
			for iter.Next() {
				res := ListRes{Key: iter.Key().Key}
				res.DataObj, _ = iter.Value()
				out <- res
			}
		}()
		return out, nil
	}
}

var ListFilterAll ListFilter = nil

func ListFilterWholeFile(r *DataObj) bool { return r.WholeFile() }

func ListByKey(fs *Basic, ks []*DbKey) (<-chan ListRes, error) {
	out := make(chan ListRes, 128)

	go func() {
		defer close(out)
		for _, k := range ks {
			res, _ := fs.GetAll(k)
			for _, kv := range res {
				out <- ListRes{Key: kv.Key.Key, DataObj: kv.Val}
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
		val, _ := itr.Value()
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

func verify(d *Basic, key *DbKey, val *DataObj, level VerifyLevel) Status {
	var err error
	switch level {
	case CheckExists:
		return StatusUnchecked
	case CheckFast:
		err = VerifyFast(val)
	case CheckIfChanged:
		_, err = GetData(d.AsFull(), key, val, VerifyIfChanged)
	case CheckAlways:
		_, err = GetData(d.AsFull(), key, val, VerifyAlways)
	default:
		return StatusError
	}

	if err == nil {
		return StatusOk
	} else if os.IsNotExist(err) {
		return StatusFileMissing
	} else if err == InvalidBlock || err == io.EOF || err == io.ErrUnexpectedEOF {
		return StatusFileChanged
	} else if err == TouchedBlock {
		return StatusFileTouched
	} else {
		return StatusFileError
	}
}

func getNodes(key *DbKey, fs *Basic, bs b.Blockstore) ([]KeyVal, []*node.Link, Status) {
	res, err := fs.GetAll(key)
	if err == nil {
		if res[0].Val.NoBlockData() {
			return res, nil, StatusUnchecked
		} else {
			links, err := GetLinks(res[0].Val)
			if err != nil {
				Logger.Errorf("%s: %v", MHash(key), err)
				return nil, nil, StatusCorrupt
			}
			return res[0:1], links, StatusOk
		}
	}
	k, err2 := key.Cid()
	if err2 != nil {
		return nil, nil, StatusError
	}
	block, err2 := bs.Get(k)
	if err == ds.ErrNotFound && err2 == b.ErrNotFound {
		return nil, nil, StatusKeyNotFound
	} else if err2 != nil {
		Logger.Errorf("%s: %v", k, err)
		Logger.Errorf("%s: %v", k, err2)
		//panic(err2)
		return nil, nil, StatusError
	}
	node, err := dag.DecodeProtobuf(block.RawData())
	if err != nil {
		Logger.Errorf("%s: %v", k, err)
		return nil, nil, StatusCorrupt
	}
	return nil, node.Links(), StatusFound
}
