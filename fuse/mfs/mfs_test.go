//go:build !nofuse && !openbsd && !netbsd && !plan9
// +build !nofuse,!openbsd,!netbsd,!plan9

package mfs

import (
	"context"
	"os"
	"strings"
	"testing"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fs/fstestutil"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/libp2p/go-libp2p-testing/ci"
)

func setUp(t *testing.T) (fs.FS, *fstestutil.Mount) {
	if ci.NoFuse() {
		t.Skip("Skipping FUSE tests")
	}

	node, err := core.NewNode(context.Background(), &node.BuildCfg{})
	if err != nil {
		t.Fatal(err)
	}
	fs := NewFileSystem(node)
	mnt, err := fstestutil.MountedT(t, fs, nil)
	if err == fuse.ErrOSXFUSENotFound {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}

	return fs, mnt
}

func TestReadWrite(t *testing.T) {
	_, mnt := setUp(t)
	defer mnt.Close()

	t.Run("write", func(t *testing.T) {
		f, err := os.Create(mnt.Dir + "/testrw")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = f.Write([]byte("test read/write"))
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		f, err := os.Open(mnt.Dir + "/testrw")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		buf := make([]byte, 15)
		l, err := f.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Compare("test read/write", string(buf[:l])) != 0 {
			t.Fatal("read and write not equal")
		}
	})
}
