package filesystem

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	gopath "path"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/djdv/p9/localfs"
	"github.com/djdv/p9/p9"
	files "github.com/ipfs/go-ipfs-files"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	fsnodes "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

var (
	attrMaskIPFSTest = p9.AttrMask{
		Mode: true,
		Size: true,
	}
	rootSubsystems = []string{"ipfs", "ipns"}
)

func TestAll(t *testing.T) {
	ctx := context.TODO()
	core, err := initCore(ctx)
	if err != nil {
		t.Fatalf("Failed to construct IPFS node: %s\n", err)
	}

	t.Run("RootFS Attach", func(t *testing.T) { testRootAttacher(ctx, t, core) })
	t.Run("RootFS Clone", func(t *testing.T) { testRootClones(ctx, t, core) })
	t.Run("RootFS", func(t *testing.T) { testRootFS(ctx, t, core) })
	t.Run("PinFS", func(t *testing.T) { testPinFS(ctx, t, core) })
	t.Run("IPFS", func(t *testing.T) { testIPFS(ctx, t, core) })
}

func testRootAttacher(ctx context.Context, t *testing.T, core coreiface.CoreAPI) {
	rootAttacher := fsnodes.RootAttacher(ctx, core)

	// 2 individual instances
	nineRoot, err := rootAttacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource: %s\n", err)
	}

	if err = nineRoot.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	nineRootTheRevenge, err := rootAttacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource a second time: %s\n", err)
	}

	if err = nineRootTheRevenge.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	// 2 instances at the same time
	nineRoot, err = rootAttacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource: %s\n", err)
	}

	nineRootTheRevenge, err = rootAttacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource a second time: %s\n", err)
	}

	if err = nineRootTheRevenge.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	if err = nineRoot.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}
}

func testRootClones(ctx context.Context, t *testing.T, core coreiface.CoreAPI) {
	nineRoot, err := fsnodes.RootAttacher(ctx, core).Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource: %s\n", err)
	}

	_, nineRef, err := nineRoot.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone root: %s\n", err)
	}

	// this shouldn't affect the parent it's derived from
	if err = nineRef.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	_, anotherNineRef, err := nineRoot.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone root: %s\n", err)
	}

	if err = anotherNineRef.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	if err = nineRoot.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

}

func testRootFS(ctx context.Context, t *testing.T, core coreiface.CoreAPI) {
	nineRoot, err := fsnodes.RootAttacher(ctx, core).Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource: %s\n", err)
	}

	_, nineRef, err := nineRoot.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to walk root: %s\n", err)
	}
	if _, _, err = nineRef.Open(p9.ReadOnly); err != nil {
		t.Fatalf("Failed to open root: %s\n", err)
	}

	ents, err := nineRef.Readdir(0, ^uint32(0))
	if err != nil {
		t.Fatalf("Failed to read root: %s\n", err)
	}

	if len(ents) != len(rootSubsystems) {
		t.Fatalf("Failed, root has bad entries:\nHave:%v\nWant:%v\n", ents, rootSubsystems)
	}
	for i, name := range rootSubsystems {
		if ents[i].Name != name {
			t.Fatalf("Failed, root has bad entries:\nHave:%v\nWant:%v\n", ents, rootSubsystems)
		}
	}
	//TODO: deeper compare than just the names / name order
}

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

func testIPFS(ctx context.Context, t *testing.T, core coreiface.CoreAPI) {
	env, iEnv, err := initEnv(ctx, core)
	if err != nil {
		t.Fatalf("Failed to construct IPFS test environment: %s\n", err)
	}
	defer os.RemoveAll(env)

	localEnv, err := localfs.Attacher(env).Attach()
	if err != nil {
		t.Fatalf("Failed to attach to local resource %q: %s\n", env, err)
	}

	ipfsRoot, err := fsnodes.IPFSAttacher(ctx, core).Attach()
	if err != nil {
		t.Fatalf("Failed to attach to IPFS resource: %s\n", err)
	}
	_, ipfsEnv, err := ipfsRoot.Walk([]string{gopath.Base(iEnv.String())})
	if err != nil {
		t.Fatalf("Failed to walk to IPFS test environment: %s\n", err)
	}
	_, envClone, err := ipfsEnv.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone IPFS environment handle: %s\n", err)
	}

	testCompareTreeAttrs(t, localEnv, ipfsEnv)

	// test readdir bounds
	//TODO: compare against a table, not just lengths
	_, _, err = envClone.Open(p9.ReadOnly)
	if err != nil {
		t.Fatalf("Failed to open IPFS test directory: %s\n", err)
	}
	ents, err := envClone.Readdir(2, 2) // start at ent 2, return max 2
	if err != nil {
		t.Fatalf("Failed to read IPFS test directory: %s\n", err)
	}
	if l := len(ents); l == 0 || l > 2 {
		t.Fatalf("IPFS test directory contents don't match read request: %v\n", ents)
	}
}

func testCompareTreeAttrs(t *testing.T, f1, f2 p9.File) {
	var expand func(p9.File) (map[string]p9.Attr, error)
	expand = func(nineRef p9.File) (map[string]p9.Attr, error) {
		ents, err := p9Readdir(nineRef)
		if err != nil {
			return nil, err
		}

		res := make(map[string]p9.Attr)
		for _, ent := range ents {
			_, child, err := nineRef.Walk([]string{ent.Name})
			if err != nil {
				return nil, err
			}

			_, _, attr, err := child.GetAttr(attrMaskIPFSTest)
			if err != nil {
				return nil, err
			}
			res[ent.Name] = attr
			//p9.AttrMaskAll

			if ent.Type == p9.TypeDir {
				subRes, err := expand(child)
				if err != nil {
					return nil, err
				}
				for name, attr := range subRes {
					res[gopath.Join(ent.Name, name)] = attr
				}
			}
		}
		return res, nil
	}

	f1Map, err := expand(f1)
	if err != nil {
		t.Fatal(err)
	}

	f2Map, err := expand(f2)
	if err != nil {
		t.Fatal(err)
	}

	same := func(permissionContains p9.FileMode, base, target map[string]p9.Attr) bool {
		if len(base) != len(target) {

			var baseNames []string
			var targetNames []string
			for name := range base {
				baseNames = append(baseNames, name)
			}
			for name := range target {
				targetNames = append(targetNames, name)
			}

			t.Fatalf("map lengths don't match:\nbase:%v\ntarget:%v\n", baseNames, targetNames)
			return false
		}

		for path, baseAttr := range base {
			bMode := baseAttr.Mode
			tMode := target[path].Mode

			if bMode.FileType() != tMode.FileType() {
				t.Fatalf("type for %q don't match:\nbase:%v\ntarget:%v\n", path, bMode, tMode)
				return false
			}

			if ((bMode.Permissions() & permissionContains) & (tMode.Permissions() & permissionContains)) == 0 {
				t.Fatalf("permissions for %q don't match\n(unfiltered)\nbase:%v\ntarget:%v\n(filtered)\nbase:%v\ntarget:%v\n",
					path,
					bMode.Permissions(), tMode.Permissions(),
					bMode.Permissions()&permissionContains, tMode.Permissions()&permissionContains,
				)
				return false
			}

			if bMode.FileType() != p9.ModeDirectory {
				bSize := baseAttr.Size
				tSize := target[path].Size

				if bSize != tSize {
					t.Fatalf("size for %q doesn't match\nbase:%d\ntarget:%d\n",
						path,
						bSize,
						tSize)
				}
			}
		}
		return true
	}
	if !same(p9.Read, f1Map, f2Map) {
		t.Fatalf("contents don't match \nf1:%v\nf2:%v\n", f1Map, f2Map)
	}
}

func initCore(ctx context.Context) (coreiface.CoreAPI, error) {
	node, err := core.NewNode(ctx, &core.BuildCfg{
		Online:                      false,
		Permanent:                   false,
		DisableEncryptedConnections: true,
	})
	if err != nil {
		return nil, err
	}

	return coreapi.NewCoreAPI(node)
}

const incantation = "May the bits passing through this device somehow help bring peace to this world"

func initEnv(ctx context.Context, core coreiface.CoreAPI) (string, corepath.Resolved, error) {
	testDir, err := ioutil.TempDir("", "ipfs-")
	if err != nil {
		return "", nil, err
	}
	if err := os.Chmod(testDir, 0775); err != nil {
		return "", nil, err
	}

	if err = ioutil.WriteFile(filepath.Join(testDir, "empty"),
		[]byte(nil),
		0644); err != nil {
		return "", nil, err
	}

	if err = ioutil.WriteFile(filepath.Join(testDir, "small"),
		[]byte(incantation),
		0644); err != nil {
		return "", nil, err
	}

	if err := generateGarbage(testDir); err != nil {
		return "", nil, err
	}

	testSubDir, err := ioutil.TempDir(testDir, "ipfs-")
	if err != nil {
		return "", nil, err
	}
	if err := os.Chmod(testSubDir, 0775); err != nil {
		return "", nil, err
	}

	if err := generateGarbage(testSubDir); err != nil {
		return "", nil, err
	}

	iPath, err := pinAddDir(ctx, core, testDir)
	if err != nil {
		return "", nil, err
	}

	return testDir, iPath, err
}

func pinAddDir(ctx context.Context, core coreiface.CoreAPI, path string) (corepath.Resolved, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	node, err := files.NewSerialFile(path, false, fi)
	if err != nil {
		return nil, err
	}

	iPath, err := core.Unixfs().Add(ctx, node.(files.Directory), coreoptions.Unixfs.Pin(true))
	if err != nil {
		return nil, err
	}
	return iPath, nil
}

func generateGarbage(tempDir string) error {
	randDev := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, size := range []int{4, 8, 16, 32} {
		buf := make([]byte, size<<(10*2))
		if _, err := randDev.Read(buf); err != nil {
			return err
		}

		name := fmt.Sprintf("%dMiB", size)
		if err := ioutil.WriteFile(filepath.Join(tempDir, name),
			buf,
			0644); err != nil {
			return err
		}
	}

	return nil
}

func pinNames(ctx context.Context, core coreiface.CoreAPI) ([]string, error) {
	pins, err := core.Pin().Ls(ctx, coreoptions.Pin.Type.Recursive())
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pins))
	for _, pin := range pins {
		names = append(names, gopath.Base(pin.Path().String()))
	}
	return names, nil
}

func p9PinNames(root p9.File) ([]string, error) {
	ents, err := p9Readdir(root)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(ents))

	for _, ent := range ents {
		names = append(names, ent.Name)
	}

	return names, nil
}

func p9Readdir(dir p9.File) ([]p9.Dirent, error) {
	_, dirClone, err := dir.Walk(nil)
	if err != nil {
		return nil, err
	}

	_, _, err = dirClone.Open(p9.ReadOnly)
	if err != nil {
		return nil, err
	}
	defer dirClone.Close()
	return dirClone.Readdir(0, ^uint32(0))
}

//TODO:
// NOTE: compares a subset of attributes, matching those of IPFS
func testIPFSCompare(t *testing.T, f1, f2 p9.File) {
	_, _, f1Attr, err := f1.GetAttr(attrMaskIPFSTest)
	if err != nil {
		t.Errorf("Attr(%v) = %v, want nil", f1, err)
	}
	_, _, f2Attr, err := f2.GetAttr(attrMaskIPFSTest)
	if err != nil {
		t.Errorf("Attr(%v) = %v, want nil", f2, err)
	}
	if f1Attr != f2Attr {
		t.Errorf("Attributes of same files do not match: %v and %v", f1Attr, f2Attr)
	}
}
