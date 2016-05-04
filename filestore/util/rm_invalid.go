package filestore_util

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	k "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
)

func RmInvalid(req cmds.Request, node *core.IpfsNode, fs *Datastore, mode string, quiet bool, dryRun bool) (io.Reader, error) {
	level := StatusFileMissing
	switch mode {
	case "changed":
		level = StatusFileChanged
	case "missing":
		level = StatusFileMissing
	case "all":
		level = StatusFileError
	default:
		return nil, errors.New("level must be one of: changed missing all")
	}
	ch, _ := List(fs, false)
	rdr, wtr := io.Pipe()
	var rmWtr io.Writer = wtr
	if quiet {
		rmWtr = ioutil.Discard
	}
	go func() {
		var toDel [][]byte
		for r := range ch {
			if !r.NoBlockData() {
				continue
			}
			r.Status = verify(fs, r.Key, r.DataObj, VerifyAlways)
			if r.Status >= level {
				toDel = append(toDel, r.RawHash())
				if !quiet {
					fmt.Fprintf(wtr, "will delete %s (part of %s)\n", r.MHash(), r.FilePath)
				}
			}
		}
		if dryRun {
			fmt.Fprintf(wtr, "Dry-run option specified.  Stopping.\n")
			fmt.Fprintf(wtr, "Would of deleted %d invalid objects.\n", len(toDel))
		} else {
			for _, key := range toDel {
				err := Delete(req, rmWtr, node, fs, k.Key(key))
				if err != nil {
					mhash := b58.Encode(key)
					msg := fmt.Sprintf("Could not delete %s: %s\n", mhash, err.Error())
					wtr.CloseWithError(errors.New(msg))
					return
				}
			}
			fmt.Fprintf(wtr, "Deleted %d invalid objects.\n", len(toDel))
		}
		wtr.Close()
	}()
	return rdr, nil
}
