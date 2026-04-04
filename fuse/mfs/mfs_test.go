//go:build !nofuse && !openbsd && !netbsd && !plan9

// Unit tests for the mutable /mfs FUSE mount.
// These test the filesystem implementation directly without a daemon.
// End-to-end tests that exercise mount/unmount through a real daemon
// live in test/cli/fuse/.

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
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/ipfs/kubo/fuse/fusetest"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
)

// Create an Ipfs.Node, a filesystem and a mount point.
func setUp(t *testing.T, ipfs *core.IpfsNode, cfgs ...config.Mounts) (fs.FS, *fstestutil.Mount) {
	fusetest.SkipUnlessFUSE(t)

	var cfg config.Mounts
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}

	if ipfs == nil {
		var err error
		ipfs, err = core.NewNode(context.Background(), &node.BuildCfg{})
		if err != nil {
			t.Fatal(err)
		}
	}

	fs := NewFileSystem(ipfs, cfg)
	mnt, err := fstestutil.MountedT(t, fs, nil)
	fusetest.MountError(t, err)

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

// Test that empty directories can be listed without errors.
func TestEmptyDirListing(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	// The MFS root starts empty.
	entries, err := os.ReadDir(mnt.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty root, got %d entries", len(entries))
	}

	// Create a directory and list it while still empty.
	dir := mnt.Dir + "/emptydir"
	if err := os.Mkdir(dir, os.ModeDir); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty directory, got %d entries", len(entries))
	}
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
		for i := range files {
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
		for range files {
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
		for range files {
			err := <-errs
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

// Test appending data to an existing file.
func TestAppendFile(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	path := mnt.Dir + "/appendfile"

	initial := make([]byte, 1300)
	if _, err := rand.Read(initial); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatal(err)
	}

	fi, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	extra := make([]byte, 500)
	if _, err := rand.Read(extra); err != nil {
		t.Fatal(err)
	}

	n, err := fi.Write(extra)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(extra) {
		t.Fatalf("short write: %d != %d", n, len(extra))
	}
	if err := fi.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append(initial, extra...)
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

// Test writing a file one byte at a time.
func TestMultiWrite(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	path := mnt.Dir + "/multiwrite"
	fi, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 1001)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}

	for i := range data {
		n, err := fi.Write(data[i : i+1])
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("short write")
		}
	}
	if err := fi.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("content mismatch")
	}
}

// Test renaming a file within the same directory.
func TestRenameFile(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	src := mnt.Dir + "/before.txt"
	dst := mnt.Dir + "/after.txt"

	data := make([]byte, 500)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}

	// Source must be gone.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source still exists after rename: %v", err)
	}

	// Destination must have the original content.
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(got), len(data))
	}
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
	if !bytes.Contains(listRes.Xattr, []byte(fusemnt.XattrCID)) {
		t.Fatalf("xattr list does not contain %s: %q", fusemnt.XattrCID, listRes.Xattr)
	}

	getReq := fuse.GetxattrRequest{
		Name: fusemnt.XattrCID,
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

// Test that StoreMtime persists mtime in UnixFS metadata across remounts.
// We verify persistence rather than stat output because the kernel tracks
// write times in its own cache regardless of what the FUSE daemon stores.
func TestStoreMtime(t *testing.T) {
	before := time.Now().Add(-time.Second)
	ipfs, err := core.NewNode(context.Background(), &node.BuildCfg{})
	if err != nil {
		t.Fatal(err)
	}
	_, mnt := setUp(t, ipfs, config.Mounts{StoreMtime: config.True})

	fname := mnt.Dir + "/withtime"
	if err := os.WriteFile(fname, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	mnt.Close()

	// Remount with default config and verify the mtime survived.
	_, mnt = setUp(t, ipfs)
	defer mnt.Close()

	fi, err := os.Stat(mnt.Dir + "/withtime")
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().After(before) {
		t.Fatalf("mtime did not persist across remount: got %v", fi.ModTime())
	}
}

// Test that the default file mode matches the shared constant and chmod
// is ignored without StoreMode.
func TestStoreMode(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		_, mnt := setUp(t, nil)
		defer mnt.Close()

		fname := mnt.Dir + "/nomode"
		if err := os.WriteFile(fname, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != fusemnt.DefaultFileModeRW.Perm() {
			t.Fatalf("expected default mode %04o, got %04o", fusemnt.DefaultFileModeRW.Perm(), fi.Mode().Perm())
		}
		_ = os.Chmod(fname, 0o755)
		fi, err = os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != fusemnt.DefaultFileModeRW.Perm() {
			t.Fatalf("mode changed without StoreMode: got %04o", fi.Mode().Perm())
		}
	})

	t.Run("enabled", func(t *testing.T) {
		_, mnt := setUp(t, nil, config.Mounts{StoreMode: config.True})
		defer mnt.Close()

		fname := mnt.Dir + "/withmode"
		if err := os.WriteFile(fname, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() == 0o755 {
			t.Fatal("new file already has 0755, cannot distinguish chmod effect")
		}
		if err := os.Chmod(fname, 0o755); err != nil {
			t.Fatal(err)
		}
		fi, err = os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o755 {
			t.Fatalf("expected mode 0755 after chmod, got %04o", fi.Mode().Perm())
		}
	})
}

// Test that directories get the expected default mode.
func TestDefaultDirMode(t *testing.T) {
	_, mnt := setUp(t, nil)
	defer mnt.Close()

	dir := mnt.Dir + "/subdir"
	if err := os.Mkdir(dir, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != fusemnt.DefaultDirModeRW.Perm() {
		t.Fatalf("expected dir mode %04o, got %04o", fusemnt.DefaultDirModeRW.Perm(), fi.Mode().Perm())
	}
}
