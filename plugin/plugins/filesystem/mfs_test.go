package filesystem

import (
	"context"
	"testing"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

func testMFS(ctx context.Context, t *testing.T, core coreiface.CoreAPI) {
	//TODO: init root CID
	//t.Run("Baseline", func(t *testing.T) { baseLine(ctx, t, core, fsnodes.MFSAttacher) })
}
