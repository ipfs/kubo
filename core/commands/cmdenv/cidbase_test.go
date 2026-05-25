package cmdenv

import (
	"testing"

	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	mbase "github.com/multiformats/go-multibase"
)

func TestGetCidEncoder(t *testing.T) {
	makeReq := func(opts map[string]any) *cmds.Request {
		if opts == nil {
			opts = map[string]any{}
		}
		return &cmds.Request{Options: opts}
	}

	t.Run("no options returns default encoder", func(t *testing.T) {
		enc, err := GetCidEncoder(makeReq(nil))
		if err != nil {
			t.Fatal(err)
		}
		if enc.Upgrade {
			t.Error("expected Upgrade=false with no options")
		}
	})

	t.Run("non-base58btc base auto-upgrades CIDv0", func(t *testing.T) {
		enc, err := GetCidEncoder(makeReq(map[string]any{
			"cid-base": "base32",
		}))
		if err != nil {
			t.Fatal(err)
		}
		if !enc.Upgrade {
			t.Error("expected Upgrade=true for base32")
		}
		if enc.Base.Encoding() != mbase.Base32 {
			t.Errorf("expected base32 encoding, got %v", enc.Base.Encoding())
		}
	})

	t.Run("base58btc does not auto-upgrade", func(t *testing.T) {
		enc, err := GetCidEncoder(makeReq(map[string]any{
			"cid-base": "base58btc",
		}))
		if err != nil {
			t.Fatal(err)
		}
		if enc.Upgrade {
			t.Error("expected Upgrade=false for base58btc")
		}
	})

	t.Run("deprecated flag still works as override", func(t *testing.T) {
		// Explicitly disable upgrade even with non-base58btc base
		enc, err := GetCidEncoder(makeReq(map[string]any{
			"cid-base":                "base32",
			"upgrade-cidv0-in-output": false,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if enc.Upgrade {
			t.Error("expected Upgrade=false when explicitly disabled")
		}

		// Explicitly enable upgrade even with base58btc
		enc, err = GetCidEncoder(makeReq(map[string]any{
			"cid-base":                "base58btc",
			"upgrade-cidv0-in-output": true,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if !enc.Upgrade {
			t.Error("expected Upgrade=true when explicitly enabled")
		}
	})
}

func TestEncoderFromPath(t *testing.T) {
	test := func(path string, expected cidenc.Encoder) {
		actual, err := CidEncoderFromPath(path)
		if err != nil {
			t.Error(err)
		}
		if actual != expected {
			t.Errorf("CidEncoderFromPath(%s) failed: expected %#v but got %#v", path, expected, actual)
		}
	}
	p := "QmRqVG8VGdKZ7KARqR96MV7VNHgWvEQifk94br5HpURpfu"
	enc := cidenc.Default()
	test(p, enc)
	test(p+"/a", enc)
	test(p+"/a/b", enc)
	test(p+"/a/b/", enc)
	test(p+"/a/b/c", enc)
	test("/ipfs/"+p, enc)
	test("/ipfs/"+p+"/b", enc)

	p = "zb2rhfkM4FjkMLaUnygwhuqkETzbYXnUDf1P9MSmdNjW1w1Lk"
	enc = cidenc.Encoder{
		Base:    mbase.MustNewEncoder(mbase.Base58BTC),
		Upgrade: true,
	}
	test(p, enc)
	test(p+"/a", enc)
	test(p+"/a/b", enc)
	test(p+"/a/b/", enc)
	test(p+"/a/b/c", enc)
	test("/ipfs/"+p, enc)
	test("/ipfs/"+p+"/b", enc)
	test("/ipld/"+p, enc)
	test("/ipns/"+p, enc) // even IPNS should work.

	p = "bafyreifrcnyjokuw4i4ggkzg534tjlc25lqgt3ttznflmyv5fftdgu52hm"
	enc = cidenc.Encoder{
		Base:    mbase.MustNewEncoder(mbase.Base32),
		Upgrade: true,
	}
	test(p, enc)
	test("/ipfs/"+p, enc)
	test("/ipld/"+p, enc)

	for _, badPath := range []string{
		"/ipld/",
		"/ipld",
		"/ipld//",
		"ipld//",
		"ipld",
		"",
		"ipns",
		"/ipfs/asdf",
		"/ipfs/...",
		"...",
		"abcdefg",
		"boo",
	} {
		_, err := CidEncoderFromPath(badPath)
		if err == nil {
			t.Errorf("expected error extracting encoder from bad path: %s", badPath)
		}
	}
}
