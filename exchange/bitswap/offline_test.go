package bitswap

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestBlockReturnsErr(t *testing.T) {
	off := NewOfflineExchange()
	_, err := off.Block(context.TODO(), u.Key("foo"))
	if err != nil {
		return // as desired
	}
	t.Fail()
}

func TestHasBlockReturnsNil(t *testing.T) {
	off := NewOfflineExchange()
	block := testutil.NewBlockOrFail(t, "data")
	err := off.HasBlock(block)
	if err != nil {
		t.Fatal("")
	}
}
