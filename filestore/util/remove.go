package filestore_util

import (
	//"fmt"
	//"io"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	u "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	. "github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/pin"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
)

type FilestoreRemover struct {
	ss                   Snapshot
	tocheck              []*cid.Cid
	ReportFound          bool
	ReportNotFound       bool
	ReportAlreadyDeleted bool
}

func NewFilestoreRemover(ss Snapshot) *FilestoreRemover {
	return &FilestoreRemover{ss: ss, ReportFound: true, ReportNotFound: true}
}

func (r *FilestoreRemover) Delete(key *DbKey) *u.RemovedBlock {
	_, dataObj, err := r.ss.GetDirect(key)
	if err == ds.ErrNotFound {
		if r.ReportNotFound {
			return &u.RemovedBlock{Hash: key.Format(), Error: err.Error()}
		} else {
			return nil
		}
	} else if err != nil {
		return &u.RemovedBlock{Hash: key.Format(), Error: err.Error()}
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
			todel, err := r.ss.GetAll(k.Bytes)
			if err != nil {
				out <- &u.RemovedBlock{Hash: k.Format(), Error: err.Error()}
			}
			for _, dataObj := range todel {
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

func RmBlocks(fs *Datastore, mbs bs.MultiBlockstore, pins pin.Pinner, out chan<- interface{}, keys []*DbKey) error {
	ss, err := fs.GetSnapshot()
	if err != nil {
		return err
	}
	r := NewFilestoreRemover(ss)
	go func() {
		defer close(out)
		for _, k := range keys {
			res := r.Delete(k)
			if res != nil {
				out <- res
			}
		}
		out2 := r.Finish(mbs, pins)
		for res := range out2 {
			out <- res
		}
	}()
	return nil
}

// 	go func() {
// 		defer close(out)

// 		// First get a snapshot

// 		tocheck := ...
// 		for _, k := range keys {
// 			RmBlockPrePinCheck(...)
// 		}

// 		unlocker := mbs.GCLock()
// 		defer unlocker.Unlock()

// 		stillOkay := FilterPinned(mbs, pins, out, tocheck, prefix)

// 		for _, c := range stillOkay {
// 			err := blocks.DeleteBlock(c)
// 			if err != nil && opts.Force && (err == bs.ErrNotFound || err == ds.ErrNotFound) {
// 				// ignore non-existent blocks
// 			} else if err != nil {
// 				out <- &u.RemovedBlock{Hash: c.String(), Error: err.Error()}
// 			} else if !opts.Quiet {
// 				out <- &u.RemovedBlock{Hash: c.String()}
// 			}
// 		}
// 	}()
// 	return nil
// }
