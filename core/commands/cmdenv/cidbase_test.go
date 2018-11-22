package cmdenv

import (
	"testing"
)

func TestExtractCidString(t *testing.T) {
	test := func(path string, cid string) {
		res, err := extractCidString(path)
		if err != nil || res != cid {
			t.Errorf("extractCidString(%s) failed", path)
		}
	}
	testFailure := func(path string) {
		_, err := extractCidString(path)
		if err == nil {
			t.Errorf("extractCidString(%s) should of failed", path)
		}
	}
	p := "QmRqVG8VGdKZ7KARqR96MV7VNHgWvEQifk94br5HpURpfu"
	test(p, p)
	test("/ipfs/"+p, p)
	testFailure("/ipns/" + p)

	p = "zb2rhfkM4FjkMLaUnygwhuqkETzbYXnUDf1P9MSmdNjW1w1Lk"
	test(p, p)
	test("/ipfs/"+p, p)
	test("/ipld/"+p, p)

	testFailure("/ipfs")
}
