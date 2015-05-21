package core

import (
	"testing"

	path "github.com/ipfs/go-ipfs/path"
	"strings"
)

func TestResolveInvalidPath(t *testing.T) {
	n, err := NewMockNode()
	if n == nil || err != nil {
		t.Fatal("Should have constructed.", err)
	}

	_, err = Resolve(n.Context(), n, path.Path("/ipfs/"))
	if !strings.HasPrefix(err.Error(), "invalid path") {
		t.Fatal("Should get invalid path.", err)
	}

}
