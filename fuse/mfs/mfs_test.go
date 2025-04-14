//go:build !nofuse && !openbsd && !netbsd && !plan9
// +build !nofuse,!openbsd,!netbsd,!plan9

package mfs

import (
	"bytes"
	"context"
	"crypto/rand"
	iofs "io/fs"
	"os"
	"testing"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fs/fstestutil"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/libp2p/go-libp2p-testing/ci"
)

// Create an Ipfs.Node, a filesystem and a mount point.
func setUp(t *testing.T, ipfs *core.IpfsNode) (fs.FS, *fstestutil.Mount) {
	if ci.NoFuse() {
		t.Skip("Skipping FUSE tests")
	}

	if ipfs == nil {
		var err error
		ipfs, err = core.NewNode(context.Background(), &node.BuildCfg{})
		if err != nil {
			t.Fatal(err)
		}
	}

	fs := NewFileSystem(ipfs)
	mnt, err := fstestutil.MountedT(t, fs, nil)
	if err == fuse.ErrOSXFUSENotFound {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}

	return fs, mnt
}

// Test reading and writing a file.
func TestReadWrite(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	path := mnt.Dir + "/testrw"
	content := make([]byte, 1024)
	_, err := rand.Read(content)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("write", func(t *testing.T) {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = f.Write(content)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		buf := make([]byte, 1024)
		l, err := f.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(content, buf[:l]) != true {
			t.Fatal("read and write not equal")
		}
	})
}

// Test creating a directory.
func TestMkdir(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	path := mnt.Dir + "/foo/bar/baz/qux/quux"

	t.Run("write", func(t *testing.T) {
		err := os.MkdirAll(path, iofs.ModeDir)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		stat, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !stat.IsDir() {
			t.Fatal("not dir")
		}
	})
}

// Test file persistence across mounts.
func TestPersistence(t *testing.T) {
	ipfs, err := core.NewNode(context.Background(), &node.BuildCfg{})
	if err != nil {
		t.Fatal(err)
	}

	content := make([]byte, 1024)
	_, err = rand.Read(content)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("write", func(t *testing.T) {
		_, mnt := setUp(t, ipfs)
		defer mnt.Close()
		path := mnt.Dir + "/testpersistence"

		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = f.Write(content)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		_, mnt := setUp(t, ipfs)
		defer mnt.Close()
		path := mnt.Dir + "/testpersistence"

		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		buf := make([]byte, 1024)
		l, err := f.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(content, buf[:l]) != true {
			t.Fatal("read and write not equal")
		}
	})
}

// Test getting the file attributes.
func TestAttr(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	path := mnt.Dir + "/testattr"
	content := make([]byte, 1024)
	_, err := rand.Read(content)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("write", func(t *testing.T) {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = f.Write(content)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}

		if fi.IsDir() {
			t.Fatal("file is a directory")
		}

		if fi.ModTime().After(time.Now()) {
			t.Fatal("future modtime")
		}
		if time.Since(fi.ModTime()) > time.Second {
			t.Fatal("past modtime")
		}

		if fi.Name() != "testattr" {
			t.Fatal("invalid filename")
		}

		if fi.Size() != 1024 {
			t.Fatal("invalid size")
		}
	})
}
