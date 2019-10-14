package filesystem

import (
	"context"
	"os"
	"sort"
	"testing"

	fsnodes "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

func testPinFS(ctx context.Context, t *testing.T, core coreiface.CoreAPI) {
	pinRoot, err := fsnodes.PinFSAttacher(ctx, core).Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P Pin resource: %s\n", err)
	}

	same := func(base, target []string) bool {
		if len(base) != len(target) {
			return false
		}
		sort.Strings(base)
		sort.Strings(target)

		for i := len(base) - 1; i >= 0; i-- {
			if target[i] != base[i] {
				return false
			}
		}
		return true
	}

	shallowCompare := func() {
		basePins, err := pinNames(ctx, core)
		if err != nil {
			t.Fatalf("Failed to list IPFS pins: %s\n", err)
		}
		p9Pins, err := p9PinNames(pinRoot)
		if err != nil {
			t.Fatalf("Failed to list 9P pins: %s\n", err)
		}

		if !same(basePins, p9Pins) {
			t.Fatalf("Pinsets differ\ncore: %v\n9P: %v\n", basePins, p9Pins)
		}
	}

	//test default (likely empty) test repo pins
	shallowCompare()

	// test modifying pinset +1; initEnv pins its IPFS environment
	env, _, err := initEnv(ctx, core)
	if err != nil {
		t.Fatalf("Failed to construct IPFS test environment: %s\n", err)
	}
	defer os.RemoveAll(env)
	shallowCompare()

	// test modifying pinset +1 again; generate garbage and pin it
	if err := generateGarbage(env); err != nil {
		t.Fatalf("Failed to generate test data: %s\n", err)
	}
	if _, err = pinAddDir(ctx, core, env); err != nil {
		t.Fatalf("Failed to add directory to IPFS: %s\n", err)
	}
	shallowCompare()
}
