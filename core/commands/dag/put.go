package dagcmd

import (
	"bytes"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	"github.com/ipld/go-ipld-prime/multicodec"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"

	"github.com/ipfs/boxo/files"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipld "github.com/ipfs/go-ipld-format"
	mc "github.com/multiformats/go-multicodec"

	// Expected minimal set of available format/ienc codecs.
	_ "github.com/ipld/go-codec-dagpb"
	_ "github.com/ipld/go-ipld-prime/codec/cbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	_ "github.com/ipld/go-ipld-prime/codec/json"
	_ "github.com/ipld/go-ipld-prime/codec/raw"
)

func dagPut(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	nd, err := cmdenv.GetNode(env)
	if err != nil {
		return err
	}

	cfg, err := nd.Repo.Config()
	if err != nil {
		return err
	}

	inputCodec, _ := req.Options["input-codec"].(string)
	storeCodec, _ := req.Options["store-codec"].(string)
	hash, _ := req.Options["hash"].(string)
	dopin, _ := req.Options["pin"].(bool)

	if hash == "" {
		hash = cfg.Import.HashFunction.WithDefault(config.DefaultHashFunction)
	}

	var icodec mc.Code
	if err := icodec.Set(inputCodec); err != nil {
		return err
	}
	var scodec mc.Code
	if err := scodec.Set(storeCodec); err != nil {
		return err
	}
	var mhType mc.Code
	if err := mhType.Set(hash); err != nil {
		return err
	}

	cidPrefix := cid.Prefix{
		Version:  1,
		Codec:    uint64(scodec),
		MhType:   uint64(mhType),
		MhLength: -1,
	}

	decoder, err := multicodec.LookupDecoder(uint64(icodec))
	if err != nil {
		return err
	}
	encoder, err := multicodec.LookupEncoder(uint64(scodec))
	if err != nil {
		return err
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

		node := basicnode.Prototype.Any.NewBuilder()
		if err := decoder(node, file); err != nil {
			return err
		}
		n := node.Build()

		bd := bytes.NewBuffer([]byte{})
		if err := encoder(n, bd); err != nil {
			return err
		}

		blockCid, err := cidPrefix.Sum(bd.Bytes())
		if err != nil {
			return err
		}
		blk, err := blocks.NewBlockWithCid(bd.Bytes(), blockCid)
		if err != nil {
			return err
		}
		ln := ipldlegacy.LegacyNode{
			Block: blk,
			Node:  n,
		}

		if err := cmdutils.CheckBlockSize(req, uint64(bd.Len())); err != nil {
			return err
		}

		if err := b.Add(req.Context, &ln); err != nil {
			return err
		}

		cid := ln.Cid()
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
