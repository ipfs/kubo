package cmdenv

import (
	"testing"
)

func TestExtractCidString(t *testing.T) {
	test := func(path string, cid string) {
		res := extractCidString(path)
		if res != cid {
			t.Errorf("extractCidString(%s) failed: expected '%s' but got '%s'", path, cid, res)
		}
	}
	p := "QmRqVG8VGdKZ7KARqR96MV7VNHgWvEQifk94br5HpURpfu"
	test(p, p)
	test("/ipfs/"+p, p)

	p = "zb2rhfkM4FjkMLaUnygwhuqkETzbYXnUDf1P9MSmdNjW1w1Lk"
	test(p, p)
	test("/ipfs/"+p, p)
	test("/ipld/"+p, p)

	p = "bafyreifrcnyjokuw4i4ggkzg534tjlc25lqgt3ttznflmyv5fftdgu52hm"
	test(p, p)
	test("/ipfs/"+p, p)
	test("/ipld/"+p, p)

	// an error is also acceptable in future versions of extractCidString
	test("/ipfs", "/ipfs")
}
