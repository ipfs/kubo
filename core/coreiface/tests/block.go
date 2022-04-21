package tests

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	ipld "github.com/ipfs/go-ipld-format"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	opt "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"

	mh "github.com/multiformats/go-multihash"
)

var (
	pbCidV0  = "QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN"                                                                 // dag-pb
	pbCid    = "bafybeiffndsajwhk3lwjewwdxqntmjm4b5wxaaanokonsggenkbw6slwk4"                                                    // dag-pb
	rawCid   = "bafkreiffndsajwhk3lwjewwdxqntmjm4b5wxaaanokonsggenkbw6slwk4"                                                    // raw bytes
	cborCid  = "bafyreicnga62zhxnmnlt6ymq5hcbsg7gdhqdu6z4ehu3wpjhvqnflfy6nm"                                                    // dag-cbor
	cborKCid = "bafyr2qgsohbwdlk7ajmmbb4lhoytmest4wdbe5xnexfvtxeatuyqqmwv3fgxp3pmhpc27gwey2cct56gloqefoqwcf3yqiqzsaqb7p4jefhcw" // dag-cbor keccak-512
)

// dag-pb
func pbBlock() io.Reader {
	return bytes.NewReader([]byte{10, 12, 8, 2, 18, 6, 104, 101, 108, 108, 111, 10, 24, 6})
}

// dag-cbor
func cborBlock() io.Reader {
	return bytes.NewReader([]byte{101, 72, 101, 108, 108, 111})
}

func (tp *TestSuite) TestBlock(t *testing.T) {
	tp.hasApi(t, func(api coreiface.CoreAPI) error {
		if api.Block() == nil {
			return errAPINotImplemented
		}
		return nil
	})

	t.Run("TestBlockPut (get raw CIDv1)", tp.TestBlockPut)
	t.Run("TestBlockPutCidCodec: dag-pb", tp.TestBlockPutCidCodecDagPb)
	t.Run("TestBlockPutCidCodec: dag-cbor", tp.TestBlockPutCidCodecDagCbor)
	t.Run("TestBlockPutFormat (legacy): cbor → dag-cbor", tp.TestBlockPutFormatDagCbor)
	t.Run("TestBlockPutFormat (legacy): protobuf → dag-pb", tp.TestBlockPutFormatDagPb)
	t.Run("TestBlockPutFormat (legacy): v0 → CIDv0", tp.TestBlockPutFormatV0)
	t.Run("TestBlockPutHash", tp.TestBlockPutHash)
	t.Run("TestBlockGet", tp.TestBlockGet)
	t.Run("TestBlockRm", tp.TestBlockRm)
	t.Run("TestBlockStat", tp.TestBlockStat)
	t.Run("TestBlockPin", tp.TestBlockPin)
}

// when no opts are passed, produced CID has 'raw' codec
func (tp *TestSuite) TestBlockPut(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, pbBlock())
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != rawCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

// Format is deprecated, it used invalid codec names.
// Confirm 'cbor' gets fixed to 'dag-cbor'
func (tp *TestSuite) TestBlockPutFormatDagCbor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, cborBlock(), opt.Block.Format("cbor"))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != cborCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

// Format is deprecated, it used invalid codec names.
// Confirm 'protobuf' got fixed to 'dag-pb'
func (tp *TestSuite) TestBlockPutFormatDagPb(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, pbBlock(), opt.Block.Format("protobuf"))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != pbCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

// Format is deprecated, it used invalid codec names.
// Confirm fake codec 'v0' got fixed to CIDv0 (with implicit dag-pb codec)
func (tp *TestSuite) TestBlockPutFormatV0(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, pbBlock(), opt.Block.Format("v0"))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != pbCidV0 {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func (tp *TestSuite) TestBlockPutCidCodecDagCbor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, cborBlock(), opt.Block.CidCodec("dag-cbor"))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != cborCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func (tp *TestSuite) TestBlockPutCidCodecDagPb(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, pbBlock(), opt.Block.CidCodec("dag-pb"))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != pbCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func (tp *TestSuite) TestBlockPutHash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(
		ctx,
		cborBlock(),
		opt.Block.Hash(mh.KECCAK_512, -1),
		opt.Block.CidCodec("dag-cbor"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != cborKCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func (tp *TestSuite) TestBlockGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("raw"))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Block().Get(ctx, res.Path())
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if string(d) != "Hello" {
		t.Error("didn't get correct data back")
	}

	p := path.New("/ipfs/" + res.Path().Cid().String())

	rp, err := api.ResolvePath(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if rp.Cid().String() != res.Path().Cid().String() {
		t.Error("paths didn't match")
	}
}

func (tp *TestSuite) TestBlockRm(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("raw"))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Block().Get(ctx, res.Path())
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if string(d) != "Hello" {
		t.Error("didn't get correct data back")
	}

	err = api.Block().Rm(ctx, res.Path())
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Block().Get(ctx, res.Path())
	if err == nil {
		t.Fatal("expected err to exist")
	}
	if !ipld.IsNotFound(err) {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res.Path())
	if err == nil {
		t.Fatal("expected err to exist")
	}
	if !ipld.IsNotFound(err) {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res.Path(), opt.Block.Force(true))
	if err != nil {
		t.Fatal(err)
	}
}

func (tp *TestSuite) TestBlockStat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("raw"))
	if err != nil {
		t.Fatal(err)
	}

	stat, err := api.Block().Stat(ctx, res.Path())
	if err != nil {
		t.Fatal(err)
	}

	if stat.Path().String() != res.Path().String() {
		t.Error("paths don't match")
	}

	if stat.Size() != len("Hello") {
		t.Error("length doesn't match")
	}
}

func (tp *TestSuite) TestBlockPin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("raw"))
	if err != nil {
		t.Fatal(err)
	}

	if pins, err := api.Pin().Ls(ctx); err != nil || len(pins) != 0 {
		t.Fatal("expected 0 pins")
	}

	res, err := api.Block().Put(
		ctx,
		strings.NewReader(`Hello`),
		opt.Block.Pin(true),
		opt.Block.Format("raw"),
	)
	if err != nil {
		t.Fatal(err)
	}

	pins, err := accPins(api.Pin().Ls(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 1 {
		t.Fatal("expected 1 pin")
	}
	if pins[0].Type() != "recursive" {
		t.Error("expected a recursive pin")
	}
	if pins[0].Path().String() != res.Path().String() {
		t.Error("pin path didn't match")
	}
}
