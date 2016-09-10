package filestore_util

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	butil "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	k "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/pin"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	//b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
)

func Clean(req cmds.Request, node *core.IpfsNode, fs *Datastore, quiet bool, what ...string) (io.Reader, error) {
	exclusiveMode := node.LocalMode()
	stage1 := false
	stage2 := false
	stage3 := false
	to_remove := make([]bool, 100)
	for i := 0; i < len(what); i++ {
		switch what[i] {
		case "invalid":
			what = append(what, "changed", "no-file")
		case "full":
			what = append(what, "invalid", "incomplete", "orphan")
		case "changed":
			stage1 = true
			to_remove[StatusFileChanged] = true
		case "no-file":
			stage1 = true
			to_remove[StatusFileMissing] = true
		case "error":
			stage1 = true
			to_remove[StatusFileError] = true
		case "incomplete":
			stage2 = true
			to_remove[StatusIncomplete] = true
		case "orphan":
			stage3 = true
			to_remove[StatusOrphan] = true
		default:
			return nil, errors.New("invalid arg: " + what[i])
		}
	}
	rdr, wtr := io.Pipe()
	var rmWtr io.Writer = wtr
	if quiet {
		rmWtr = ioutil.Discard
	}
	Logger.Debugf("Starting clean operation.")
	snapshot, err := fs.GetSnapshot()
	if err != nil {
		return nil, err
	}
	do_stage := func(ch <-chan ListRes, err error) error {
		if err != nil {
			wtr.CloseWithError(err)
			return err
		}
		var toDel []k.Key
		for r := range ch {
			if to_remove[r.Status] {
				dsKey, err := k.KeyFromDsKey(r.Key)
				if err != nil {
					wtr.CloseWithError(err)
					return err
				}
				toDel = append(toDel, dsKey)
			}
		}
		ch2 := make(chan interface{}, 16)
		if exclusiveMode {
			rmBlocks(node.Blockstore, node.Pinning, ch2, toDel, Snapshot{}, fs)
		} else {
			rmBlocks(node.Blockstore, node.Pinning, ch2, toDel, snapshot, fs)
		}
		err2 := butil.ProcRmOutput(ch2, rmWtr, wtr)
		if err2 != nil && err2.Fatal {
			wtr.CloseWithError(err2)
			return err2
		}
		return nil
	}
	go func() {
		if stage1 {
			fmt.Fprintf(rmWtr, "Scanning for invalid leaf nodes ('verify --basic -l6') ...\n")
			err := do_stage(VerifyBasic(snapshot, 6, 1))
			if err != nil {
				return
			}
		}
		if stage2 {
			fmt.Fprintf(rmWtr, "Scanning for incomplete nodes ('verify -l1 --skip-orphans') ...\n")
			err := do_stage(VerifyFull(node, snapshot, 1, 1, true))
			if err != nil {
				return
			}
		}
		if stage3 {
			fmt.Fprintf(rmWtr, "Scanning for orphans ('verify -l1') ...\n")
			err := do_stage(VerifyFull(node, snapshot, 1, 1, false))
			if err != nil {
				return
			}
		}
		wtr.Close()
	}()
	return rdr, nil
}

func rmBlocks(mbs bs.MultiBlockstore, pins pin.Pinner, out chan<- interface{}, keys []k.Key,
	snap Snapshot, fs *Datastore) {

	debugCleanRmDelay()

	if snap.Defined() {
		Logger.Debugf("Removing invalid blocks after clean.  Online Mode.")
	} else {
		Logger.Debugf("Removing invalid blocks after clean.  Exclusive Mode.")
	}

	prefix := fsrepo.FilestoreMount

	go func() {
		defer close(out)

		unlocker := mbs.GCLock()
		defer unlocker.Unlock()

		stillOkay := butil.CheckPins(mbs, pins, out, keys, prefix)

		for _, k := range stillOkay {
			keyBytes := k.DsKey().Bytes()
			var origVal []byte
			if snap.Defined() {
				var err error
				origVal, err = snap.DB().Get(keyBytes, nil)
				if err != nil {
					out <- &butil.RemovedBlock{Hash: k.String(), Error: err.Error()}
					continue
				}
			}
			ok, err := fs.Update(keyBytes, origVal, nil)
			// Update does not return an error if the key no longer exist
			if err != nil {
				out <- &butil.RemovedBlock{Hash: k.String(), Error: err.Error()}
			} else if !ok {
				out <- &butil.RemovedBlock{Hash: k.String(), Error: "Value Changed"}
			} else {
				out <- &butil.RemovedBlock{Hash: k.String()}
			}
		}
	}()
}

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
