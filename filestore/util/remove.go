package filestore_util

import (
	//"fmt"
	//"io"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	u "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	. "github.com/ipfs/go-ipfs/filestore"
	. "github.com/ipfs/go-ipfs/filestore/support"
	"github.com/ipfs/go-ipfs/pin"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
)

type FilestoreRemover struct {
	ss                   Snapshot
	tocheck              []*cid.Cid
	ReportFound          bool
	ReportNotFound       bool
	ReportAlreadyDeleted bool
	Recursive            bool
	AllowNonRoots        bool
}

// Batch removal of filestore blocks works on a snapshot of the
// database and is done in two passes.

// In the first pass a DataObj is deleted if it can be done so without
// requiring a pin check.  If a pin check is required than the hash is
// appened to the tocheck for the pin check.
//
// By definition if a hash in in tocheck there is only one DataObj for
// that hash left at the time the snapshot was taken.
//
// In the second pass the pincheck is done and any unpinned hashes are
// deleted.  In the case that there is multiple DataObjs in the
// snapshot all but one will have already been removed; however we
// can't easily tell which one so just re-delete all the keys present
// in the snapshot.

func NewFilestoreRemover(ss Snapshot) *FilestoreRemover {
	return &FilestoreRemover{ss: ss, ReportFound: true, ReportNotFound: true}
}

func (r *FilestoreRemover) Delete(key *DbKey, dataObj *DataObj) *u.RemovedBlock {
	var err error
	if dataObj == nil {
		_, dataObj, err = r.ss.GetDirect(key)
		if err == ds.ErrNotFound {
			if r.ReportNotFound {
				return &u.RemovedBlock{Hash: key.Format(), Error: err.Error()}
			} else {
				return nil
			}
		} else if err != nil {
			return &u.RemovedBlock{Hash: key.Format(), Error: err.Error()}
		}
	}
	fullKey := key.MakeFull(dataObj)
	err = r.ss.AsFull().DelSingle(fullKey, MaybePinned)
	if err == ds.ErrNotFound {
		if r.ReportAlreadyDeleted {
			return &u.RemovedBlock{Hash: fullKey.Format(), Error: "already deleted"}
		} else {
			return nil
		}
	} else if err == RequirePinCheck {
		c, _ := fullKey.Cid()
		r.tocheck = append(r.tocheck, c)
		return nil
	} else if err != nil {
		return &u.RemovedBlock{Hash: fullKey.Format(), Error: err.Error()}
	} else {
		if r.ReportFound {
			return &u.RemovedBlock{Hash: fullKey.Format()}
		} else {
			return nil
		}
	}
}

func (r *FilestoreRemover) DeleteAll(key *DbKey, out chan<- interface{}) {
	kvs, err := r.ss.GetAll(key)
	if err == ds.ErrNotFound {
		if r.ReportNotFound {
			out <- &u.RemovedBlock{Hash: key.Format(), Error: err.Error()}
		}
	} else if err != nil {
		out <- &u.RemovedBlock{Hash: key.Format(), Error: err.Error()}
	}
	requireRoot := !r.AllowNonRoots && key.FilePath == ""
	for _, kv := range kvs {
		fullKey := kv.Key.MakeFull(kv.Val)
		if requireRoot && !kv.Val.WholeFile() {
			out <- &u.RemovedBlock{Hash: fullKey.Format(), Error: "key is not a root and no file path was specified"}
			continue
		}
		if r.Recursive {
			r.DeleteRec(kv.Key, kv.Val, out)
		} else {
			res := r.Delete(kv.Key, kv.Val)
			if res != nil {
				out <- res
			}
		}
	}
}

func (r *FilestoreRemover) DeleteRec(key *DbKey, dataObj *DataObj, out chan<- interface{}) {
	res := r.Delete(key, dataObj)
	if res != nil {
		out <- res
	}
	filePath := dataObj.FilePath
	links, err := GetLinks(dataObj)
	if err != nil {
		out <- &u.RemovedBlock{Hash: key.Format(), Error: err.Error()}
		return
	}
	for _, link := range links {
		// only delete entries that have a matching FilePath
		k := NewDbKey(dshelp.CidToDsKey(link.Cid).String(), filePath, -1, link.Cid)
		kvs, err := r.ss.GetAll(k)
		if err == ds.ErrNotFound {
			if r.ReportNotFound {
				out <- &u.RemovedBlock{Hash: k.Format(), Error: err.Error()}
			}
		} else if err != nil {
			out <- &u.RemovedBlock{Hash: k.Format(), Error: err.Error()}
		}
		for _, kv := range kvs {
			r.DeleteRec(kv.Key, kv.Val, out)
		}
	}
}

func (r *FilestoreRemover) Finish(mbs bs.MultiBlockstore, pins pin.Pinner) <-chan interface{} {
	// make the channel large enough to hold any result to avoid
	// blocking while holding the GCLock
	out := make(chan interface{}, len(r.tocheck))
	prefix := fsrepo.FilestoreMount

	if len(r.tocheck) == 0 {
		close(out)
		return out
	}

	go func() {
		defer close(out)

		unlocker := mbs.GCLock()
		defer unlocker.Unlock()

		stillOkay := u.FilterPinned(mbs, pins, out, r.tocheck, prefix)

		for _, c := range stillOkay {
			k := CidToKey(c)
			todel, err := r.ss.GetAll(k)
			if err != nil {
				out <- &u.RemovedBlock{Hash: k.Format(), Error: err.Error()}
			}
			for _, kv := range todel {
				dataObj := kv.Val
				dbKey := k.MakeFull(dataObj)
				err = r.ss.AsFull().DelDirect(dbKey, NotPinned)
				if err == ds.ErrNotFound {
					if r.ReportAlreadyDeleted {
						out <- &u.RemovedBlock{Hash: dbKey.Format(), Error: "already deleted"}
					}
				} else if err != nil {
					out <- &u.RemovedBlock{Hash: dbKey.Format(), Error: err.Error()}
				} else {
					if r.ReportFound {
						out <- &u.RemovedBlock{Hash: dbKey.Format()}
					}
				}
			}
		}
	}()

	return out
}

// func RmBlocks(fs *Datastore, mbs bs.MultiBlockstore, pins pin.Pinner, out chan<- interface{}, keys []*DbKey) error {
// 	ss,err := fs.GetSnapshot()
// 	if err != nil {
// 		return err
// 	}
// 	r := NewFilestoreRemover(ss)
// 	go func() {
// 		defer close(out)
// 		for _, k := range keys {
// 			res := r.Delete(k)
// 			if res != nil {
// 				out <- res
// 			}
// 		}
// 		out2 := r.Finish(mbs, pins)
// 		for res := range out2 {
// 			out <- res
// 		}
// 	}()
// 	return nil
// }
