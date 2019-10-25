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
	t.Run("Baseline", func(t *testing.T) { baseLine(ctx, t, core, fsnodes.PinFSAttacher) })

	pinRoot, err := fsnodes.PinFSAttacher(ctx, core).Attach()
	if err != nil {
		t.Logf("Failed to attach to 9P Pin resource: %s\n", err)
		t.FailNow()
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
			t.Logf("Failed to list IPFS pins: %s\n", err)
			t.FailNow()
		}
		p9Pins, err := p9PinNames(pinRoot)
		if err != nil {
			t.Logf("Failed to list 9P pins: %s\n", err)
			t.FailNow()
		}

		if !same(basePins, p9Pins) {
			t.Logf("Pinsets differ\ncore: %v\n9P: %v\n", basePins, p9Pins)
			t.FailNow()
		}
	}

	//test default (likely empty) test repo pins
	shallowCompare()

	// test modifying pinset +1; initEnv pins its IPFS environment
	env, _, err := initEnv(ctx, core)
	if err != nil {
		t.Logf("Failed to construct IPFS test environment: %s\n", err)
		t.FailNow()
	}
	defer os.RemoveAll(env)
	shallowCompare()

	// test modifying pinset +1 again; generate garbage and pin it
	if err := generateGarbage(env); err != nil {
		t.Logf("Failed to generate test data: %s\n", err)
		t.FailNow()
	}
	if _, err = pinAddDir(ctx, core, env); err != nil {
		t.Logf("Failed to add directory to IPFS: %s\n", err)
		t.FailNow()
	}
	shallowCompare()
}
