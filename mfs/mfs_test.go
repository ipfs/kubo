package mfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"testing"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	u "github.com/ipfs/go-ipfs/util"
)

type dagservAndPinner struct {
	ds dag.DAGService
	mp pin.ManualPinner
}

func getDagservAndPinner(t *testing.T) dagservAndPinner {
	db := dssync.MutexWrap(ds.NewMapDatastore())
	bs := bstore.NewBlockstore(db)
	blockserv := bserv.New(bs, offline.Exchange(bs))
	dserv := dag.NewDAGService(blockserv)
	mpin := pin.NewPinner(db, dserv).GetManual()
	return dagservAndPinner{
		ds: dserv,
		mp: mpin,
	}
}

func getRandFile(t *testing.T, ds dag.DAGService, size int64) *dag.Node {
	r := io.LimitReader(u.NewTimeSeededRand(), size)
	nd, err := importer.BuildDagFromReader(ds, chunk.DefaultSplitter(r), nil)
	if err != nil {
		t.Fatal(err)
	}
	return nd
}

func mkdirP(t *testing.T, root *Directory, path string) *Directory {
	dirs := strings.Split(path, "/")
	cur := root
	for _, d := range dirs {
		n, err := cur.Mkdir(d)
		if err != nil && err != os.ErrExist {
			t.Fatal(err)
		}
		if err == os.ErrExist {
			fsn, err := cur.Child(d)
			if err != nil {
				t.Fatal(err)
			}
			switch fsn := fsn.(type) {
			case *Directory:
				n = fsn
			case *File:
				t.Fatal("tried to make a directory where a file already exists")
			}
		}

		cur = n
	}
	return cur
}

func assertDirAtPath(root *Directory, path string, children []string) error {
	fsn, err := DirLookup(root, path)
	if err != nil {
		return err
	}

	dir, ok := fsn.(*Directory)
	if !ok {
		return fmt.Errorf("%s was not a directory", path)
	}

	listing, err := dir.List()
	if err != nil {
		return err
	}

	var names []string
	for _, d := range listing {
		names = append(names, d.Name)
	}

	sort.Strings(children)
	sort.Strings(names)
	if !compStrArrs(children, names) {
		return errors.New("directories children did not match!")
	}

	return nil
}

func compStrArrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func assertFileAtPath(ds dag.DAGService, root *Directory, exp *dag.Node, path string) error {
	parts := strings.Split(path, "/")
	cur := root
	for i, d := range parts[:len(parts)-1] {
		next, err := cur.Child(d)
		if err != nil {
			return fmt.Errorf("looking for %s failed: %s", path, err)
		}

		nextDir, ok := next.(*Directory)
		if !ok {
			return fmt.Errorf("%s points to a non-directory", parts[:i+1])
		}

		cur = nextDir
	}

	last := parts[len(parts)-1]
	finaln, err := cur.Child(last)
	if err != nil {
		return err
	}

	file, ok := finaln.(*File)
	if !ok {
		return fmt.Errorf("%s was not a file!", path)
	}

	out, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	expbytes, err := catNode(ds, exp)
	if err != nil {
		return err
	}

	if !bytes.Equal(out, expbytes) {
		return fmt.Errorf("Incorrect data at path!")
	}
	return nil
}

func catNode(ds dag.DAGService, nd *dag.Node) ([]byte, error) {
	r, err := uio.NewDagReader(context.TODO(), nd, ds)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return ioutil.ReadAll(r)
}

func setupFsAndRoot(ctx context.Context, t *testing.T, rootname string) (*dagservAndPinner, *Filesystem, *Root) {
	dsp := getDagservAndPinner(t)

	fs, err := NewFilesystem(ctx, dsp.ds, dsp.mp)
	if err != nil {
		t.Fatal(err)
	}

	root := &dag.Node{Data: ft.FolderPBData()}
	rt, err := fs.NewRoot("test", root, func(ctx context.Context, k key.Key) error {
		fmt.Println("PUBLISHED: ", k)
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	return &dsp, fs, rt
}

func TestBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dsp, _, rt := setupFsAndRoot(ctx, t, "test")

	rootdir := rt.GetValue().(*Directory)

	// test making a basic dir
	_, err := rootdir.Mkdir("a")
	if err != nil {
		t.Fatal(err)
	}

	path := "a/b/c/d/e/f/g"
	d := mkdirP(t, rootdir, path)

	fi := getRandFile(t, dsp.ds, 1000)

	// test inserting that file
	err = d.AddChild("afile", fi)
	if err != nil {
		t.Fatal(err)
	}

	err = assertFileAtPath(dsp.ds, rootdir, fi, "a/b/c/d/e/f/g/afile")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMkdir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _, rt := setupFsAndRoot(ctx, t, "test")

	rootdir := rt.GetValue().(*Directory)

	dirsToMake := []string{"a", "B", "foo", "bar", "cats", "fish"}
	sort.Strings(dirsToMake) // sort for easy comparing later

	for _, d := range dirsToMake {
		_, err := rootdir.Mkdir(d)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := assertDirAtPath(rootdir, "/", dirsToMake)
	if err != nil {
		t.Fatal(err)
	}

	for _, d := range dirsToMake {
		mkdirP(t, rootdir, "a/"+d)
	}

	err = assertDirAtPath(rootdir, "/a", dirsToMake)
	if err != nil {
		t.Fatal(err)
	}

	// mkdir over existing dir should fail
	_, err = rootdir.Mkdir("a")
	if err == nil {
		t.Fatal("should have failed!")
	}
}

func TestFilesystemMethods(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dsp := getDagservAndPinner(t)

	fs, err := NewFilesystem(ctx, dsp.ds, dsp.mp)
	if err != nil {
		t.Fatal(err)
	}

	empty := &dag.Node{Data: ft.FolderPBData()}
	pf := func(_ context.Context, _ key.Key) error { return nil }

	roots := []string{"a", "b", "c", "d", "e"}
	sort.Strings(roots)
	for _, r := range roots {
		_, err := fs.NewRoot(r, empty, pf)
		if err != nil {
			t.Fatal(err)
		}
	}

	// make sure they all show up in the listing
	var rootstrs []string
	for _, r := range fs.ListRoots() {
		rootstrs = append(rootstrs, r.Name)
	}

	sort.Strings(rootstrs)

	if !compStrArrs(rootstrs, roots) {
		t.Fatal("didnt get expected roots in fs")
	}

	// make sure we can 'get' each of them
	for _, r := range roots {
		_, err := fs.GetRoot(r)
		if err != nil {
			t.Fatal(err)
		}
	}

	// make sure we *cant* get ones that dont exist
	_, err = fs.GetRoot("NOTREAL")
	if err != ErrNotExist {
		t.Fatal("expected ErrNotExist, got: ", err)
	}

	// make sure we cant make a root that already exists
	_, err = fs.NewRoot("a", empty, pf)
	if err == nil {
		t.Fatal("expected create already existing root to fail")
	}

	// closing a root works
	final, err := fs.CloseRoot("b")
	if err != nil {
		t.Fatal(err)
	}

	ek, _ := empty.Key()
	if final != ek {
		t.Fatal("final key was not as expected")
	}

	// closing filesystem succeeds
	err = fs.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryLoadFromDag(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dsp, _, rt := setupFsAndRoot(ctx, t, "test")

	rootdir := rt.GetValue().(*Directory)

	nd := getRandFile(t, dsp.ds, 1000)
	_, err := dsp.ds.Add(nd)
	if err != nil {
		t.Fatal(err)
	}

	fihash, err := nd.Multihash()
	if err != nil {
		t.Fatal(err)
	}

	dir := &dag.Node{Data: ft.FolderPBData()}
	_, err = dsp.ds.Add(dir)
	if err != nil {
		t.Fatal(err)
	}

	dirhash, err := dir.Multihash()
	if err != nil {
		t.Fatal(err)
	}

	top := &dag.Node{
		Data: ft.FolderPBData(),
		Links: []*dag.Link{
			&dag.Link{
				Name: "a",
				Hash: fihash,
			},
			&dag.Link{
				Name: "b",
				Hash: dirhash,
			},
		},
	}

	err = rootdir.AddChild("foo", top)
	if err != nil {
		t.Fatal(err)
	}

	// get this dir
	topi, err := rootdir.Child("foo")
	if err != nil {
		t.Fatal(err)
	}

	topd := topi.(*Directory)

	// mkdir over existing but unloaded child file should fail
	_, err = topd.Mkdir("a")
	if err == nil {
		t.Fatal("expected to fail!")
	}

	// mkdir over existing but unloaded child dir should fail
	_, err = topd.Mkdir("b")
	if err == nil {
		t.Fatal("expected to fail!")
	}

	// adding a child over an existing path fails
	err = topd.AddChild("b", nd)
	if err == nil {
		t.Fatal("expected to fail!")
	}

	err = assertFileAtPath(dsp.ds, rootdir, nd, "foo/a")
	if err != nil {
		t.Fatal(err)
	}

	err = assertDirAtPath(rootdir, "foo/b", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = rootdir.Unlink("foo")
	if err != nil {
		t.Fatal(err)
	}

	err = assertDirAtPath(rootdir, "", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMfsFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dsp, _, rt := setupFsAndRoot(ctx, t, "test")

	rootdir := rt.GetValue().(*Directory)

	fisize := 1000
	nd := getRandFile(t, dsp.ds, 1000)

	err := rootdir.AddChild("file", nd)
	if err != nil {
		t.Fatal(err)
	}

	fsn, err := rootdir.Child("file")
	if err != nil {
		t.Fatal(err)
	}

	fi := fsn.(*File)

	if fi.Type() != TFile {
		t.Fatal("some is seriously wrong here")
	}

	// assert size is as expected
	size, err := fi.Size()
	if size != int64(fisize) {
		t.Fatal("size isnt correct")
	}

	// write to beginning of file
	b := []byte("THIS IS A TEST")
	n, err := fi.Write(b)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(b) {
		t.Fatal("didnt write correct number of bytes")
	}

	// sync file
	err = fi.Sync()
	if err != nil {
		t.Fatal(err)
	}

	// make sure size hasnt changed
	size, err = fi.Size()
	if size != int64(fisize) {
		t.Fatal("size isnt correct")
	}

	// seek back to beginning
	ns, err := fi.Seek(0, os.SEEK_SET)
	if err != nil {
		t.Fatal(err)
	}

	if ns != 0 {
		t.Fatal("didnt seek to beginning")
	}

	// read back bytes we wrote
	buf := make([]byte, len(b))
	n, err = fi.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(buf) {
		t.Fatal("didnt read enough")
	}

	if !bytes.Equal(buf, b) {
		t.Fatal("data read was different than data written")
	}

	// truncate file to ten bytes
	err = fi.Truncate(10)
	if err != nil {
		t.Fatal(err)
	}

	size, err = fi.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != 10 {
		t.Fatal("size was incorrect: ", size)
	}

	// 'writeAt' to extend it
	data := []byte("this is a test foo foo foo")
	nwa, err := fi.WriteAt(data, 5)
	if err != nil {
		t.Fatal(err)
	}

	if nwa != len(data) {
		t.Fatal(err)
	}

	// assert size once more
	size, err = fi.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != int64(5+len(data)) {
		t.Fatal("size was incorrect")
	}

	// make sure we can get node. TODO: verify it later
	_, err = fi.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	// close it out!
	err = fi.Close()
	if err != nil {
		t.Fatal(err)
	}
}
