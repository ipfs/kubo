package filesystem

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	gopath "path"
	"path/filepath"
	"testing"
	"time"

	"github.com/djdv/p9/p9"
	files "github.com/ipfs/go-ipfs-files"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

var attrMaskIPFSTest = p9.AttrMask{
	Mode: true,
	Size: true,
}

func TestAll(t *testing.T) {
	ctx := context.TODO()
	core, err := InitCore(ctx)
	if err != nil {
		t.Fatalf("Failed to construct IPFS node: %s\n", err)
	}

	t.Run("RootFS", func(t *testing.T) { testRootFS(ctx, t, core) })
	t.Run("PinFS", func(t *testing.T) { testPinFS(ctx, t, core) })
	t.Run("IPFS", func(t *testing.T) { testIPFS(ctx, t, core) })
}

type baseAttacher func(context.Context, coreiface.CoreAPI, ...nodeopts.AttachOption) p9.Attacher

func baseLine(ctx context.Context, t *testing.T, core coreiface.CoreAPI, attachFn baseAttacher) {
	attacher := attachFn(ctx, core)

	t.Run("attacher", func(t *testing.T) { testAttacher(ctx, t, attacher) })
	root, err := attacher.Attach()
	if err != nil {
		t.Fatalf("Attach test passed but attach failed: %s\n", err)
	}
	t.Run("walk", func(t *testing.T) { testClones(ctx, t, root) })

}

func testAttacher(ctx context.Context, t *testing.T, attacher p9.Attacher) {
	// 2 individual instances, one after another
	nineRoot, err := attacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource: %s\n", err)
	}

	if err = nineRoot.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	nineRootTheRevenge, err := attacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource a second time: %s\n", err)
	}

	if err = nineRootTheRevenge.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	// 2 instances at the same time
	nineRoot, err = attacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource: %s\n", err)
	}

	nineRootTheRevenge, err = attacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource a second time: %s\n", err)
	}

	if err = nineRootTheRevenge.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	if err = nineRoot.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	// final instance
	nineRoot, err = attacher.Attach()
	if err != nil {
		t.Fatalf("Failed to attach to 9P root resource: %s\n", err)
	}

	if err = nineRoot.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}
}

func testClones(ctx context.Context, t *testing.T, nineRef p9.File) {
	// clone the node we were passed; 1st generation
	_, newRef, err := nineRef.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone root: %s\n", err)
	}

	// this `Close` shouldn't affect the parent it's derived from
	// only descendants
	if err = newRef.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	// remake the clone from the original; 1st generation again
	_, gen1, err := nineRef.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone root: %s\n", err)
	}

	// clone a 2nd generation from the 1st
	_, gen2, err := gen1.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone root: %s\n", err)
	}

	// 3rd from the 2nd
	_, gen3, err := gen2.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone root: %s\n", err)
	}

	// close the 2nd reference
	if err = gen2.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	// try to clone from the 2nd reference
	// this should fail since we closed it
	_, undead, err := gen2.Walk(nil)
	if err == nil {
		t.Fatalf("Clone (%p)%q succeeded when parent (%p)%q was closed\n", undead, undead, gen2, gen2)
	}

	// 4th from  the 3rd
	// should still succeed regardless of 2's state
	_, gen4, err := gen3.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone root: %s\n", err)
	}

	// close the 3rd reference
	if err = gen3.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	// close the 4th reference
	if err = gen4.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	// clone a 2nd generation from the 1st again
	_, gen2, err = gen1.Walk(nil)
	if err != nil {
		t.Fatalf("Failed to clone root: %s\n", err)
	}

	// close the 1st
	if err = gen1.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
	}

	// close the 2nd
	if err = gen2.Close(); err != nil {
		t.Fatalf("Close errored: %s\n", err)
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

			if ent.Type == p9.TypeDir {
				subRes, err := expand(child)
				if err != nil {
					return nil, err
				}
				for name, attr := range subRes {
					res[gopath.Join(ent.Name, name)] = attr
				}
			}
			if err = child.Close(); err != nil {
				return nil, err
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

func InitCore(ctx context.Context) (coreiface.CoreAPI, error) {
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

	var (
		offset uint64
		ents   []p9.Dirent
	)
	for {
		curEnts, err := dirClone.Readdir(offset, ^uint32(0))
		ents = append(ents, curEnts...)
		if err != nil {
			break
		}

		offset += uint64(len(curEnts))
	}

	if err == io.EOF {
		err = nil
	}

	return ents, err
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
