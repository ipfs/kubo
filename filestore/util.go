package filestore

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	pb "github.com/ipfs/go-ipfs/filestore/pb"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	blockstore "gx/ipfs/QmadMhXJLHMFjpRmh85XjpmVDkEtQpNYEZNRpWRvYVLrvb/go-ipfs-blockstore"
	dshelp "gx/ipfs/Qmd8UZEDddMaCnQ1G5eSrUhN3coX19V7SyXNQGWnAvUsnT/go-ipfs-ds-help"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
	dsq "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore/query"
)

// listWorkers is the number of workers uses when listing/verifying all objects
// in the filestore.
var listWorkers = runtime.NumCPU() * 2

// Status is used to identify the state of the block data referenced
// by a FilestoreNode. Among other places, it is used by CorruptReferenceError.
type Status int32

// These are the supported Status codes.
const (
	StatusOk           Status = 0
	StatusFileError    Status = 10 // Backing File Error
	StatusFileNotFound Status = 11 // Backing File Not Found
	StatusFileChanged  Status = 12 // Contents of the file changed
	StatusOtherError   Status = 20 // Internal Error, likely corrupt entry
	StatusKeyNotFound  Status = 30
)

// String provides a human-readable representation for Status codes.
func (s Status) String() string {
	switch s {
	case StatusOk:
		return "ok"
	case StatusFileError:
		return "error"
	case StatusFileNotFound:
		return "no-file"
	case StatusFileChanged:
		return "changed"
	case StatusOtherError:
		return "ERROR"
	case StatusKeyNotFound:
		return "missing"
	default:
		return "???"
	}
}

// Format returns the status formatted as a string
// with leading 0s.
func (s Status) Format() string {
	return fmt.Sprintf("%-7s", s.String())
}

// ListRes wraps the response of the List*() functions, which
// allows to obtain and verify blocks stored by the FileManager
// of a Filestore. It includes information about the referenced
// block.
type ListRes struct {
	Status   Status
	ErrorMsg string
	Key      *cid.Cid
	FilePath string
	Offset   uint64
	Size     uint64
}

// FormatLong returns a human readable string for a ListRes object.
func (r *ListRes) FormatLong() string {
	switch {
	case r.Key == nil:
		return "<corrupt key>"
	case r.FilePath == "":
		return r.Key.String()
	default:
		return fmt.Sprintf("%-50s %6d %s %d", r.Key, r.Size, r.FilePath, r.Offset)
	}
}

// List fetches the block with the given key from the Filemanager
// of the given Filestore and returns a ListRes object with the information.
// List does not verify that the reference is valid or whether the
// raw data is accesible. See Verify().
func List(fs *Filestore, key *cid.Cid) *ListRes {
	return list(fs, false, key)
}

// ListAll returns a function as an iterator which, once invoked, returns
// one by one each block in the Filestore's FileManager.
// ListAll does not verify that the references are valid or whether
// the raw data is accessible. See VerifyAll().
func ListAll(ctx context.Context, fs *Filestore) (<-chan *ListRes, error) {
	return listAll(ctx, fs, false)
}

// Verify fetches the block with the given key from the Filemanager
// of the given Filestore and returns a ListRes object with the information.
// Verify makes sure that the reference is valid and the block data can be
// read.
func Verify(fs *Filestore, key *cid.Cid) *ListRes {
	return list(fs, true, key)
}

// VerifyAll returns a function as an iterator which, once invoked,
// returns one by one each block in the Filestore's FileManager.
// VerifyAll checks that the reference is valid and that the block data
// can be read.
func VerifyAll(ctx context.Context, fs *Filestore) (<-chan *ListRes, error) {
	return listAll(ctx, fs, true)
}

func list(fs *Filestore, verify bool, key *cid.Cid) *ListRes {
	dobj, err := fs.fm.getDataObj(key)
	if err != nil {
		return mkListRes(key, nil, err)
	}
	if verify {
		_, err = fs.fm.readDataObj(key, dobj)
	}
	return mkListRes(key, dobj, err)
}

func listAll(ctx context.Context, fs *Filestore, verify bool) (<-chan *ListRes, error) {
	q := dsq.Query{}
	qr, err := fs.fm.ds.Query(q)
	if err != nil {
		return nil, err
	}

	output := make(chan *ListRes, 64)
	var wg sync.WaitGroup
	wg.Add(listWorkers)
	qrCh := qr.Next()
	for i := 0; i < listWorkers; i++ {
		go func() {
			defer wg.Done()
			for r := range qrCh {
				cid, dobj, err := unmarshalQueryResult(r)
				if err == nil && verify {
					_, err = fs.fm.readDataObj(cid, dobj)
				}
				select {
				case output <- mkListRes(cid, dobj, err):
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(output)
	}()
	return output, nil
}

func unmarshalQueryResult(v dsq.Result) (*cid.Cid, *pb.DataObj, error) {
	if v.Error != nil {
		return nil, nil, v.Error
	}
	k := ds.RawKey(v.Key)
	c, err := dshelp.DsKeyToCid(k)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding cid from filestore: %s", err)
	}

	dobj, err := unmarshalDataObj(v.Value)
	if err != nil {
		return c, nil, err
	}

	return c, dobj, nil
}

func mkListRes(c *cid.Cid, d *pb.DataObj, err error) *ListRes {
	status := StatusOk
	errorMsg := ""
	if err != nil {
		if err == ds.ErrNotFound || err == blockstore.ErrNotFound {
			status = StatusKeyNotFound
		} else if err, ok := err.(*CorruptReferenceError); ok {
			status = err.Code
		} else {
			status = StatusOtherError
		}
		errorMsg = err.Error()
	}
	if d == nil {
		return &ListRes{
			Status:   status,
			ErrorMsg: errorMsg,
			Key:      c,
		}
	}

	return &ListRes{
		Status:   status,
		ErrorMsg: errorMsg,
		Key:      c,
		FilePath: *d.FilePath,
		Size:     *d.Size_,
		Offset:   *d.Offset,
	}
}
