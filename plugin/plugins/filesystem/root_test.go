package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/djdv/p9/p9"
	fsnodes "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var rootSubsystems = []p9.Dirent{
	{
		Name:   "ipfs",
		Offset: 1,
		Type:   p9.TypeDir,
		QID: p9.QID{
			Type: p9.TypeDir,
		},
	}, {
		Name:   "ipns",
		Offset: 2,
		Type:   p9.TypeDir,
		QID: p9.QID{
			Type: p9.TypeDir,
		},
	},
}

func testRootFS(ctx context.Context, t *testing.T, core coreiface.CoreAPI) {
	t.Run("Baseline", func(t *testing.T) { baseLine(ctx, t, core, fsnodes.RootAttacher) })

	rootRef, err := fsnodes.RootAttacher(ctx, core).Attach()
	if err != nil {
		t.Fatalf("Baseline test passed but attach failed: %s\n", err)
	}
	_, root, err := rootRef.Walk(nil)
	if err != nil {
		t.Fatalf("Baseline test passed but walk failed: %s\n", err)
	}

	t.Run("Root directory entries", func(t *testing.T) { testRootDir(ctx, t, root) })
}

func testRootDir(ctx context.Context, t *testing.T, root p9.File) {
	root.Open(p9.ReadOnly)

	ents, err := root.Readdir(0, uint32(len(rootSubsystems)))
	if err != nil {
		t.Fatal(err)
	}

	if _, err = root.Readdir(uint64(len(ents)), ^uint32(0)); err != io.EOF {
		t.Fatal(errors.New("entry count mismatch"))
	}

	for i, ent := range ents {
		// TODO: for now we trust the QID from the server
		// we should generate these paths separately during init
		rootSubsystems[i].QID.Path = ent.QID.Path

		if ent != rootSubsystems[i] {
			t.Fatal(fmt.Errorf("ent %v != expected %v", ent, rootSubsystems[i]))
		}
	}

}
