package resolve_test

import (
	"testing"

	coremock "github.com/ipfs/go-ipfs/core/mock"
	"github.com/ipfs/go-ipfs/namesys/resolve"

	path "github.com/ipfs/go-path"
)

func TestResolveNoComponents(t *testing.T) {
	n, err := coremock.NewMockNode()
	if n == nil || err != nil {
		t.Fatal("Should have constructed a mock node", err)
	}

	_, err = resolve.Resolve(n.Context(), n.Namesys, n.Resolver, path.Path("/ipns/"))
	if err.Error() != "invalid path \"/ipns/\": ipns path missing IPNS ID" {
		t.Error("Should error with no components (/ipns/).", err)
	}

	_, err = resolve.Resolve(n.Context(), n.Namesys, n.Resolver, path.Path("/ipfs/"))
	if err.Error() != "invalid path \"/ipfs/\": not enough path components" {
		t.Error("Should error with no components (/ipfs/).", err)
	}

	_, err = resolve.Resolve(n.Context(), n.Namesys, n.Resolver, path.Path("/../.."))
	if err.Error() != "invalid path \"/../..\": unknown namespace \"..\"" {
		t.Error("Should error with invalid path.", err)
	}
}
