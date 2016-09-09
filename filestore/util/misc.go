package filestore_util

import (
	"fmt"
	"io"

	. "github.com/ipfs/go-ipfs/filestore"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	butil "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	k "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

func Dups(wtr io.Writer, fs *Basic, bs b.MultiBlockstore, pins pin.Pinner, args ...string) error {
	showPinned, showUnpinned := false, false
	if len(args) == 0 {
		showPinned, showUnpinned = true, true
	}
	for _, arg := range args {
		switch arg {
		case "pinned":
			showPinned = true
		case "unpinned":
			showUnpinned = true
		default:
			return fmt.Errorf("invalid arg: %s", arg)
		}
	}
	ls := ListKeys(fs)
	dups := make([]k.Key, 0)
	for res := range ls {
		key, err := k.KeyFromDsKey(res.Key)
		if err != nil {
			return err
		}
		if butil.AvailableElsewhere(bs, fsrepo.FilestoreMount, key) {
			dups = append(dups, key)
		}
	}
	if showPinned && showUnpinned {
		for _, key := range dups {
			fmt.Fprintf(wtr, "%s\n", key)
		}
		return nil
	}
	res, err := pins.CheckIfPinned(dups...)
	if err != nil {
		return err
	}
	for _, r := range res {
		if showPinned && r.Pinned() {
			fmt.Fprintf(wtr, "%s\n", r.Key)
		} else if showUnpinned && !r.Pinned() {
			fmt.Fprintf(wtr, "%s\n", r.Key)
		}
	}
	return nil
}
