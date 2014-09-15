package bitswap

import (
	"testing"
	"time"

	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestBlockReturnsErr(t *testing.T) {
	off := NewOfflineExchange()
	_, err := off.Block(u.Key("foo"), time.Second)
	if err != nil {
		return // as desired
	}
	t.Fail()
}

func TestHasBlockReturnsNil(t *testing.T) {
	off := NewOfflineExchange()
	block := testutil.NewBlockOrFail(t, "data")
	err := off.HasBlock(&block)
	if err != nil {
		t.Fatal("")
	}
}
