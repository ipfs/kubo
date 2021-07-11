package dagcmd

import (
	"fmt"
	"math"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/coredag"

	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
)

func dagPut(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	ienc, _ := req.Options["input-enc"].(string)
	format, _ := req.Options["format"].(string)
	hash, _ := req.Options["hash"].(string)
	dopin, _ := req.Options["pin"].(bool)

	// mhType tells inputParser which hash should be used. MaxUint64 means 'use
	// default hash' (sha256 for cbor, sha1 for git..)
	mhType := uint64(math.MaxUint64)

	if hash != "" {
		var ok bool
		mhType, ok = mh.Names[hash]
		if !ok {
			return fmt.Errorf("%s in not a valid multihash name", hash)
		}
	}

	var adder ipld.NodeAdder = api.Dag()
	if dopin {
		adder = api.Dag().Pinning()
	}
	b := ipld.NewBatch(req.Context, adder)

	it := req.Files.Entries()
	for it.Next() {
		file := files.FileFromEntry(it)
		if file == nil {
			return fmt.Errorf("expected a regular file")
		}
		nds, err := coredag.ParseInputs(ienc, format, file, mhType, -1)
		if err != nil {
			return err
		}
		if len(nds) == 0 {
			return fmt.Errorf("no node returned from ParseInputs")
		}

		for _, nd := range nds {
			err := b.Add(req.Context, nd)
			if err != nil {
				return err
			}
		}

		cid := nds[0].Cid()
		if err := res.Emit(&OutputObject{Cid: cid}); err != nil {
			return err
		}
	}
	if it.Err() != nil {
		return it.Err()
	}

	if err := b.Commit(); err != nil {
		return err
	}

	return nil
}
