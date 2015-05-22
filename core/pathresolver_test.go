package core

import (
	"testing"

	path "github.com/ipfs/go-ipfs/path"
)

func TestResolveNoComponents(t *testing.T) {
	n, err := NewMockNode()
	if n == nil || err != nil {
		t.Fatal("Should have constructed a mock node", err)
	}

	_, err = Resolve(n.Context(), n, path.Path("/ipns/"))
	if err != path.ErrNoComponents {
		t.Fatal("Should error with no components (/ipns/).", err)
	}

	_, err = Resolve(n.Context(), n, path.Path("/ipfs/"))
	if err != path.ErrNoComponents {
		t.Fatal("Should error with no components (/ipfs/).", err)
	}

}
