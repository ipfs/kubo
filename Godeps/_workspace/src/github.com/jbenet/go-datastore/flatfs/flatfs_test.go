package flatfs_test

import (
	"encoding/base32"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rand "github.com/dustin/randbo"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/flatfs"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
	dstest "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/test"
)

func tempdir(t testing.TB) (path string, cleanup func()) {
	path, err := ioutil.TempDir("", "test-datastore-flatfs-")
	if err != nil {
		t.Fatalf("cannot create temp directory: %v", err)
	}

	cleanup = func() {
		if err := os.RemoveAll(path); err != nil {
			t.Errorf("tempdir cleanup failed: %v", err)
		}
	}
	return path, cleanup
}

func TestBadPrefixLen(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	for i := 0; i > -3; i-- {
		_, err := flatfs.New(temp, 0)
		if g, e := err, flatfs.ErrBadPrefixLen; g != e {
			t.Errorf("expected ErrBadPrefixLen, got: %v", g)
		}
	}
}

func TestPutBadValueType(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	err = fs.Put(datastore.NewKey("quux"), 22)
	if g, e := err, datastore.ErrInvalidType; g != e {
		t.Fatalf("expected ErrInvalidType, got: %v\n", g)
	}
}

func TestPut(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	err = fs.Put(datastore.NewKey("quux"), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}
}

func TestGet(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	const input = "foobar"
	err = fs.Put(datastore.NewKey("quux"), []byte(input))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	data, err := fs.Get(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	buf, ok := data.([]byte)
	if !ok {
		t.Fatalf("expected []byte from Get, got %T: %v", data, data)
	}
	if g, e := string(buf), input; g != e {
		t.Fatalf("Get gave wrong content: %q != %q", g, e)
	}
}

func TestPutOverwrite(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	const (
		loser  = "foobar"
		winner = "xyzzy"
	)
	err = fs.Put(datastore.NewKey("quux"), []byte(loser))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	err = fs.Put(datastore.NewKey("quux"), []byte(winner))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	data, err := fs.Get(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if g, e := string(data.([]byte)), winner; g != e {
		t.Fatalf("Get gave wrong content: %q != %q", g, e)
	}
}

func TestGetNotFoundError(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	_, err = fs.Get(datastore.NewKey("quux"))
	if g, e := err, datastore.ErrNotFound; g != e {
		t.Fatalf("expected ErrNotFound, got: %v\n", g)
	}
}

func TestStorage(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	const prefixLen = 2
	const prefix = "7175"
	const target = prefix + string(os.PathSeparator) + "71757578.data"
	fs, err := flatfs.New(temp, prefixLen)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	err = fs.Put(datastore.NewKey("quux"), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	seen := false
	walk := func(absPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		path, err := filepath.Rel(temp, absPath)
		if err != nil {
			return err
		}
		switch path {
		case ".", "..":
			// ignore
		case prefix:
			if !fi.IsDir() {
				t.Errorf("prefix directory is not a file? %v", fi.Mode())
			}
			// we know it's there if we see the file, nothing more to
			// do here
		case target:
			seen = true
			if !fi.Mode().IsRegular() {
				t.Errorf("expected a regular file, mode: %04o", fi.Mode())
			}
			if runtime.GOOS != "windows" {
				if g, e := fi.Mode()&os.ModePerm&0007, os.FileMode(0000); g != e {
					t.Errorf("file should not be world accessible: %04o", fi.Mode())
				}
			}
		default:
			t.Errorf("saw unexpected directory entry: %q %v", path, fi.Mode())
		}
		return nil
	}
	if err := filepath.Walk(temp, walk); err != nil {
		t.Fatal("walk: %v", err)
	}
	if !seen {
		t.Error("did not see the data file")
	}
}

func TestHasNotFound(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	found, err := fs.Has(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Has fail: %v\n", err)
	}
	if g, e := found, false; g != e {
		t.Fatalf("wrong Has: %v != %v", g, e)
	}
}

func TestHasFound(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	err = fs.Put(datastore.NewKey("quux"), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	found, err := fs.Has(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Has fail: %v\n", err)
	}
	if g, e := found, true; g != e {
		t.Fatalf("wrong Has: %v != %v", g, e)
	}
}

func TestDeleteNotFound(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	err = fs.Delete(datastore.NewKey("quux"))
	if g, e := err, datastore.ErrNotFound; g != e {
		t.Fatalf("expected ErrNotFound, got: %v\n", g)
	}
}

func TestDeleteFound(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	err = fs.Put(datastore.NewKey("quux"), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	err = fs.Delete(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Delete fail: %v\n", err)
	}

	// check that it's gone
	_, err = fs.Get(datastore.NewKey("quux"))
	if g, e := err, datastore.ErrNotFound; g != e {
		t.Fatalf("expected Get after Delete to give ErrNotFound, got: %v\n", g)
	}
}

func TestQuerySimple(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	const myKey = "quux"
	err = fs.Put(datastore.NewKey(myKey), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	res, err := fs.Query(query.Query{KeysOnly: true})
	if err != nil {
		t.Fatalf("Query fail: %v\n", err)
	}
	entries, err := res.Rest()
	if err != nil {
		t.Fatalf("Query Results.Rest fail: %v\n", err)
	}
	seen := false
	for _, e := range entries {
		switch e.Key {
		case datastore.NewKey(myKey).String():
			seen = true
		default:
			t.Errorf("saw unexpected key: %q", e.Key)
		}
	}
	if !seen {
		t.Errorf("did not see wanted key %q in %+v", myKey, entries)
	}
}

func TestBatchPut(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	dstest.RunBatchTest(t, fs)
}

func TestBatchDelete(t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	dstest.RunBatchDeleteTest(t, fs)
}

func BenchmarkConsecutivePut(b *testing.B) {
	r := rand.New()
	var blocks [][]byte
	var keys []datastore.Key
	for i := 0; i < b.N; i++ {
		blk := make([]byte, 256*1024)
		r.Read(blk)
		blocks = append(blocks, blk)

		key := base32.StdEncoding.EncodeToString(blk[:8])
		keys = append(keys, datastore.NewKey(key))
	}
	temp, cleanup := tempdir(b)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		b.Fatalf("New fail: %v\n", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := fs.Put(keys[i], blocks[i])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchedPut(b *testing.B) {
	r := rand.New()
	var blocks [][]byte
	var keys []datastore.Key
	for i := 0; i < b.N; i++ {
		blk := make([]byte, 256*1024)
		r.Read(blk)
		blocks = append(blocks, blk)

		key := base32.StdEncoding.EncodeToString(blk[:8])
		keys = append(keys, datastore.NewKey(key))
	}
	temp, cleanup := tempdir(b)
	defer cleanup()

	fs, err := flatfs.New(temp, 2)
	if err != nil {
		b.Fatalf("New fail: %v\n", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; {
		batch, err := fs.Batch()
		if err != nil {
			b.Fatal(err)
		}

		for n := i; i-n < 512 && i < b.N; i++ {
			err := batch.Put(keys[i], blocks[i])
			if err != nil {
				b.Fatal(err)
			}
		}
		err = batch.Commit()
		if err != nil {
			b.Fatal(err)
		}
	}
}
