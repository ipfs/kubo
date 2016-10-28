package filestore_util

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	//bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	butil "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	//"github.com/ipfs/go-ipfs/pin"
	//fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	//dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	//cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
)

func Clean(req cmds.Request, node *core.IpfsNode, fs *Datastore, quiet bool, level int, what ...string) (io.Reader, error) {
	//exclusiveMode := node.LocalMode()
	stages := 0
	to_remove := make([]bool, 100)
	incompleteWhen := make([]string, 0)
	for i := 0; i < len(what); i++ {
		switch what[i] {
		case "invalid":
			what = append(what, "changed", "no-file")
		case "full":
			what = append(what, "invalid", "incomplete", "orphan")
		case "changed":
			stages |= 0100
			incompleteWhen = append(incompleteWhen, "changed")
			to_remove[StatusFileChanged] = true
		case "no-file":
			stages |= 0100
			incompleteWhen = append(incompleteWhen, "no-file")
			to_remove[StatusFileMissing] = true
		case "error":
			stages |= 0100
			incompleteWhen = append(incompleteWhen, "error")
			to_remove[StatusFileError] = true
		case "incomplete":
			stages |= 0020
			to_remove[StatusIncomplete] = true
		case "orphan":
			stages |= 0003
			to_remove[StatusOrphan] = true
		default:
			return nil, errors.New("invalid arg: " + what[i])
		}
	}
	incompleteWhenStr := strings.Join(incompleteWhen, ",")

	rdr, wtr := io.Pipe()
	var rmWtr io.Writer = wtr
	if quiet {
		rmWtr = ioutil.Discard
	}

	snapshot, err := fs.GetSnapshot()
	if err != nil {
		return nil, err
	}

	Logger.Debugf("Starting clean operation.")

	go func() {
		// 123: verify-post-orphan required
		// 12-: verify-full
		// 1-3: verify-full required (verify-post-orphan would be incorrect)
		// 1--: basic
		// -23: verify-post-orphan required
		// -2-: verify-full (cache optional)
		// --3: verify-full required (verify-post-orphan would be incorrect)
		// ---: nothing to do!
		var ch <-chan ListRes
		switch stages {
		case 0100:
			fmt.Fprintf(rmWtr, "performing verify --basic --level=%d\n", level)
			ch, err = VerifyBasic(snapshot.Basic, &VerifyParams{
				Level:     level,
				Verbose:   1,
				NoObjInfo: true,
			})
		case 0120, 0103, 0003:
			fmt.Fprintf(rmWtr, "performing verify --level=%d --incomplete-when=%s\n",
				level, incompleteWhenStr)
			ch, err = VerifyFull(node, snapshot, &VerifyParams{
				Level:          level,
				Verbose:        6,
				IncompleteWhen: incompleteWhen,
				NoObjInfo:      true,
			})
		case 0020:
			fmt.Fprintf(rmWtr, "performing verify --skip-orphans --level=1\n")
			ch, err = VerifyFull(node, snapshot, &VerifyParams{
				SkipOrphans: true,
				Level:       level,
				Verbose:     6,
				NoObjInfo:   true,
			})
		case 0123, 0023:
			fmt.Fprintf(rmWtr, "performing verify --post-orphans --level=%d --incomplete-when=%s\n",
				level, incompleteWhenStr)
			ch, err = VerifyFull(node, snapshot, &VerifyParams{
				Level:          level,
				Verbose:        6,
				IncompleteWhen: incompleteWhen,
				PostOrphan:     true,
				NoObjInfo:      true,
			})
		default:
			// programmer error
			panic(fmt.Errorf("invalid stage string %d", stages))
		}
		if err != nil {
			wtr.CloseWithError(err)
			return
		}

		remover := NewFilestoreRemover(snapshot)

		ch2 := make(chan interface{}, 16)
		go func() {
			defer close(ch2)
			for r := range ch {
				if to_remove[r.Status] {
					r2 := remover.Delete(KeyToKey(r.Key))
					if r2 != nil {
						ch2 <- r2
					}
				}
			}
		}()
		err2 := butil.ProcRmOutput(ch2, rmWtr, wtr)
		if err2 != nil {
			wtr.CloseWithError(err2)
			return
		}
		debugCleanRmDelay()
		Logger.Debugf("Removing invalid blocks after clean.  Online Mode.")
		ch3 := remover.Finish(node.Blockstore, node.Pinning)
		err2 = butil.ProcRmOutput(ch3, rmWtr, wtr)
		if err2 != nil {
			wtr.CloseWithError(err2)
			return
		}
		wtr.Close()
	}()

	return rdr, nil
}

// func rmBlocks(mbs bs.MultiBlockstore, pins pin.Pinner, keys []*cid.Cid, snap Snapshot, fs *Datastore) <-chan interface{} {

// 	// make the channel large enough to hold any result to avoid
// 	// blocking while holding the GCLock
// 	out := make(chan interface{}, len(keys))

// 	debugCleanRmDelay()

// 	if snap.Defined() {
// 		Logger.Debugf("Removing invalid blocks after clean.  Online Mode.")
// 	} else {
// 		Logger.Debugf("Removing invalid blocks after clean.  Exclusive Mode.")
// 	}

// 	prefix := fsrepo.FilestoreMount

// 	go func() {
// 		defer close(out)

// 		unlocker := mbs.GCLock()
// 		defer unlocker.Unlock()

// 		stillOkay := butil.FilterPinned(mbs, pins, out, keys, prefix)

// 		for _, k := range stillOkay {
// 			dbKey := NewDbKey(dshelp.CidToDsKey(k).String(), "", -1, k)
// 			var err error
// 			if snap.Defined() {
// 				origVal, err0 := snap.DB().Get(dbKey)
// 				if err0 != nil {
// 					out <- &butil.RemovedBlock{Hash: dbKey.Format(), Error: err.Error()}
// 					continue
// 				}
// 				dbKey = NewDbKey(dbKey.Hash, origVal.FilePath, int64(origVal.Offset), k)
// 				err = fs.DelDirect(dbKey, NotPinned)
// 			} else {
// 				// we have an exclusive lock
// 				err = fs.DB().Delete(dbKey.Bytes)
// 			}
// 			if err != nil {
// 				out <- &butil.RemovedBlock{Hash: dbKey.Format(), Error: err.Error()}
// 			} else {
// 				out <- &butil.RemovedBlock{Hash: dbKey.Format()}
// 			}
// 		}
// 	}()

// 	return out
// }

// this function is used for testing in order to test for race
// conditions
func debugCleanRmDelay() {
	delayStr := os.Getenv("IPFS_FILESTORE_CLEAN_RM_DELAY")
	if delayStr == "" {
		return
	}
	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		Logger.Warningf("Invalid value for IPFS_FILESTORE_CLEAN_RM_DELAY: %f", delay)
	}
	println("sleeping...")
	time.Sleep(delay)
}
