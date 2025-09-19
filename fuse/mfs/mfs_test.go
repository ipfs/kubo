//go:build !nofuse && !openbsd && !netbsd && !plan9

package mfs

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	iofs "io/fs"
	"os"
	"slices"
	"strconv"
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
	content := make([]byte, 8196)
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

		buf := make([]byte, 8196)
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

	content := make([]byte, 8196)
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

		buf := make([]byte, 8196)
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
	content := make([]byte, 8196)
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

		if fi.Size() != 8196 {
			t.Fatal("invalid size")
		}
	})
}

// Test concurrent access to the filesystem.
func TestConcurrentRW(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	files := 5
	fileWorkers := 5

	path := mnt.Dir + "/testconcurrent"
	content := make([][]byte, files)

	for i := range content {
		content[i] = make([]byte, 8196)
		_, err := rand.Read(content[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Run("write", func(t *testing.T) {
		errs := make(chan (error), 1)
		for i := 0; i < files; i++ {
			go func() {
				var err error
				defer func() { errs <- err }()

				f, err := os.Create(path + strconv.Itoa(i))
				if err != nil {
					return
				}
				defer f.Close()

				_, err = f.Write(content[i])
				if err != nil {
					return
				}
			}()
		}
		for i := 0; i < files; i++ {
			err := <-errs
			if err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("read", func(t *testing.T) {
		errs := make(chan (error), 1)
		for i := 0; i < files*fileWorkers; i++ {
			go func() {
				var err error
				defer func() { errs <- err }()

				f, err := os.Open(path + strconv.Itoa(i/fileWorkers))
				if err != nil {
					return
				}
				defer f.Close()

				buf := make([]byte, 8196)
				l, err := f.Read(buf)
				if err != nil {
					return
				}
				if bytes.Equal(content[i/fileWorkers], buf[:l]) != true {
					err = errors.New("read and write not equal")
					return
				}
			}()
		}
		for i := 0; i < files; i++ {
			err := <-errs
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

// Test ipfs_cid extended attribute
func TestMFSRootXattr(t *testing.T) {
	ipfs, err := core.NewNode(context.Background(), &node.BuildCfg{})
	if err != nil {
		t.Fatal(err)
	}

	fs, mnt := setUp(t, ipfs)
	defer mnt.Close()

	node, err := fs.Root()
	if err != nil {
		t.Fatal(err)
	}

	root := node.(*Dir)

	listReq := fuse.ListxattrRequest{}
	listRes := fuse.ListxattrResponse{}
	err = root.Listxattr(context.Background(), &listReq, &listRes)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Compare(listRes.Xattr, []byte("ipfs_cid\x00")) != 0 {
		t.Fatal("list xattr returns invalid value")
	}

	getReq := fuse.GetxattrRequest{
		Name: "ipfs_cid",
	}
	getRes := fuse.GetxattrResponse{}
	err = root.Getxattr(context.Background(), &getReq, &getRes)
	if err != nil {
		t.Fatal(err)
	}

	ipldNode, err := ipfs.FilesRoot.GetDirectory().GetNode()
	if err != nil {
		t.Fatal(err)
	}

	if slices.Compare(getRes.Xattr, []byte(ipldNode.Cid().String())) != 0 {
		t.Fatal("xattr cid not equal to mfs root cid")
	}
}
