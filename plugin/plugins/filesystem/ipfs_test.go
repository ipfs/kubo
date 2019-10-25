package filesystem

import (
	"context"
	"os"
	gopath "path"
	"testing"

	"github.com/hugelgupf/p9/localfs"
	"github.com/hugelgupf/p9/p9"
	fsnodes "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

func testIPFS(ctx context.Context, t *testing.T, core coreiface.CoreAPI) {
	t.Run("Baseline", func(t *testing.T) { baseLine(ctx, t, core, fsnodes.IPFSAttacher) })

	rootRef, err := fsnodes.IPFSAttacher(ctx, core).Attach()
	if err != nil {
		t.Logf("Baseline test passed but attach failed: %s\n", err)
		t.FailNow()
	}

	env, iEnv, err := initEnv(ctx, core)
	if err != nil {
		t.Logf("Failed to construct IPFS test environment: %s\n", err)
		t.FailNow()
	}
	defer os.RemoveAll(env)

	localEnv, err := localfs.Attacher(env).Attach()
	if err != nil {
		t.Logf("Failed to attach to local resource %q: %s\n", env, err)
		t.FailNow()
	}

	_, ipfsEnv, err := rootRef.Walk([]string{gopath.Base(iEnv.String())})
	if err != nil {
		t.Logf("Failed to walk to IPFS test environment: %s\n", err)
		t.FailNow()
	}
	_, envClone, err := ipfsEnv.Walk(nil)
	if err != nil {
		t.Logf("Failed to clone IPFS environment handle: %s\n", err)
		t.FailNow()
	}

	testCompareTreeAttrs(t, localEnv, ipfsEnv)

	// test readdir bounds
	//TODO: compare against a table, not just lengths
	_, _, err = envClone.Open(p9.ReadOnly)
	if err != nil {
		t.Logf("Failed to open IPFS test directory: %s\n", err)
		t.FailNow()
	}
	ents, err := envClone.Readdir(2, 2) // start at ent 2, return max 2
	if err != nil {
		t.Logf("Failed to read IPFS test directory: %s\n", err)
		t.FailNow()
	}
	if l := len(ents); l == 0 || l > 2 {
		t.Logf("IPFS test directory contents don't match read request: %v\n", ents)
		t.FailNow()
	}
}
