package cmdenv

import (
	"testing"

	cidenc "github.com/ipfs/go-cidutil/cidenc"
	mbase "github.com/multiformats/go-multibase"
)

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
