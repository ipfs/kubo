package filestore

import (
	"fmt"
	"io"
	"os"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/query"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
)

const (
	StatusOk      = 1
	StatusError   = 2
	StatusMissing = 3
	StatusChanged = 4
)

func statusStr(status int) string {
	switch status {
	case 0:
		return ""
	case StatusOk:
		return "ok       "
	case StatusError:
		return "error    "
	case StatusMissing:
		return "missing  "
	case StatusChanged:
		return "changed  "
	default:
		return "??       "
	}
}

type ListRes struct {
	Key ds.Key
	*DataObj
	Status int
}

func (r *ListRes) MHash() string {
	return b58.Encode(r.Key.Bytes()[1:])
}

func (r *ListRes) RawHash() []byte {
	return r.Key.Bytes()[1:]
}

func (r *ListRes) Format() string {
	mhash := r.MHash()
	return fmt.Sprintf("%s%s %s\n", statusStr(r.Status), mhash, r.DataObj.Format())
}

func List(d *Datastore, keysOnly bool) (<-chan ListRes, error) {
	qr, err := d.Query(query.Query{KeysOnly: true})
	if err != nil {
		return nil, err
	}

	bufSize := 128
	if keysOnly {
		bufSize = 1024
	}
	out := make (chan ListRes, bufSize)

	go func() {
		defer close(out)
		for r := range qr.Next() {
			if r.Error != nil {
				return // FIXMEx
			}
			key := ds.NewKey(r.Key)
			if (keysOnly) {
				out <- ListRes{key, nil, 0}
			} else {
				val, _ := d.GetDirect(key)
				out <- ListRes{key, val, 0}
			}
		}
	}()
	return out, nil
}

func Verify(d *Datastore, key ds.Key, val *DataObj) int {
	status := 0
	_, err := d.GetData(key, val, VerifyAlways, true)
	if err == nil {
		status = StatusOk
	} else if os.IsNotExist(err) {
		status = StatusMissing
	} else if _, ok := err.(InvalidBlock); ok || err == io.EOF || err == io.ErrUnexpectedEOF {
		status = StatusChanged
	} else {
		status = StatusError
	}
	return status
}

