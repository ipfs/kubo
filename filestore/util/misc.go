package filestore_util

import (
	"fmt"
	"io"

	. "github.com/ipfs/go-ipfs/filestore"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	butil "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	k "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"
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
	dups := make([]*cid.Cid, 0)
	for res := range ls {
		key, err := k.KeyFromDsKey(res.Key)
		if err != nil {
			return err
		}
		c := cid.NewCidV0(key.ToMultihash())
		if butil.AvailableElsewhere(bs, fsrepo.FilestoreMount, c) {
			dups = append(dups, c)
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
