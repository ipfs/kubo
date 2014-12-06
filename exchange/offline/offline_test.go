package offline

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

func TestBlockReturnsErr(t *testing.T) {
	off := Exchange()
	_, err := off.GetBlock(context.Background(), u.Key("foo"))
	if err != nil {
		return // as desired
	}
	t.Fail()
}

func TestHasBlockReturnsNil(t *testing.T) {
	off := Exchange()
	block := blocks.NewBlock([]byte("data"))
	err := off.HasBlock(context.Background(), block)
	if err != nil {
		t.Fatal("")
	}
}
