package mfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"
	"testing"

	randbo "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/dustin/randbo"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-ipfs/path"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	u "github.com/ipfs/go-ipfs/util"
)

func getDagserv(t *testing.T) dag.DAGService {
	db := dssync.MutexWrap(ds.NewMapDatastore())
	bs := bstore.NewBlockstore(db)
	blockserv := bserv.New(bs, offline.Exchange(bs))
	return dag.NewDAGService(blockserv)
}

func getRandFile(t *testing.T, ds dag.DAGService, size int64) *dag.Node {
	r := io.LimitReader(u.NewTimeSeededRand(), size)
	nd, err := importer.BuildDagFromReader(ds, chunk.DefaultSplitter(r))
	if err != nil {
		t.Fatal(err)
	}
	return nd
}

func mkdirP(t *testing.T, root *Directory, pth string) *Directory {
	dirs := path.SplitList(pth)
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

func assertDirAtPath(root *Directory, pth string, children []string) error {
	fsn, err := DirLookup(root, pth)
	if err != nil {
		return err
	}

	dir, ok := fsn.(*Directory)
	if !ok {
		return fmt.Errorf("%s was not a directory", pth)
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

func assertFileAtPath(ds dag.DAGService, root *Directory, exp *dag.Node, pth string) error {
	parts := path.SplitList(pth)
	cur := root
	for i, d := range parts[:len(parts)-1] {
		next, err := cur.Child(d)
		if err != nil {
			return fmt.Errorf("looking for %s failed: %s", pth, err)
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
		return fmt.Errorf("%s was not a file!", pth)
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

func setupRoot(ctx context.Context, t *testing.T) (dag.DAGService, *Root) {
	ds := getDagserv(t)

	root := &dag.Node{Data: ft.FolderPBData()}
	rt, err := NewRoot(ctx, ds, root, func(ctx context.Context, k key.Key) error {
		fmt.Println("PUBLISHED: ", k)
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	return ds, rt
}

func TestBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ds, rt := setupRoot(ctx, t)

	rootdir := rt.GetValue().(*Directory)

	// test making a basic dir
	_, err := rootdir.Mkdir("a")
	if err != nil {
		t.Fatal(err)
	}

	path := "a/b/c/d/e/f/g"
	d := mkdirP(t, rootdir, path)

	fi := getRandFile(t, ds, 1000)

	// test inserting that file
	err = d.AddChild("afile", fi)
	if err != nil {
		t.Fatal(err)
	}

	err = assertFileAtPath(ds, rootdir, fi, "a/b/c/d/e/f/g/afile")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMkdir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, rt := setupRoot(ctx, t)

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

func TestDirectoryLoadFromDag(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ds, rt := setupRoot(ctx, t)

	rootdir := rt.GetValue().(*Directory)

	nd := getRandFile(t, ds, 1000)
	_, err := ds.Add(nd)
	if err != nil {
		t.Fatal(err)
	}

	fihash, err := nd.Multihash()
	if err != nil {
		t.Fatal(err)
	}

	dir := &dag.Node{Data: ft.FolderPBData()}
	_, err = ds.Add(dir)
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

	err = assertFileAtPath(ds, rootdir, nd, "foo/a")
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
	ds, rt := setupRoot(ctx, t)

	rootdir := rt.GetValue().(*Directory)

	fisize := 1000
	nd := getRandFile(t, ds, 1000)

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

func randomWalk(d *Directory, n int) (*Directory, error) {
	for i := 0; i < n; i++ {
		dirents, err := d.List()
		if err != nil {
			return nil, err
		}

		var childdirs []NodeListing
		for _, child := range dirents {
			if child.Type == int(TDir) {
				childdirs = append(childdirs, child)
			}
		}
		if len(childdirs) == 0 {
			return d, nil
		}

		next := childdirs[rand.Intn(len(childdirs))].Name

		nextD, err := d.Child(next)
		if err != nil {
			return nil, err
		}

		d = nextD.(*Directory)
	}
	return d, nil
}

func randomName() string {
	set := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890_"
	length := rand.Intn(10) + 2
	var out string
	for i := 0; i < length; i++ {
		j := rand.Intn(len(set))
		out += set[j : j+1]
	}
	return out
}

func actorMakeFile(d *Directory) error {
	d, err := randomWalk(d, rand.Intn(7))
	if err != nil {
		return err
	}

	name := randomName()
	f, err := NewFile(name, &dag.Node{Data: ft.FilePBData(nil, 0)}, d, d.dserv)
	if err != nil {
		return err
	}

	r := io.LimitReader(randbo.New(), int64(77*rand.Intn(123)))
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	return nil
}
func actorMkdir(d *Directory) error {
	d, err := randomWalk(d, rand.Intn(7))
	if err != nil {
		return err
	}

	_, err = d.Mkdir(randomName())
	if err != nil {
		return err
	}

	return nil
}

func actorRemoveFile(d *Directory) error {
	d, err := randomWalk(d, rand.Intn(7))
	if err != nil {
		return err
	}

	ents, err := d.List()
	if err != nil {
		return err
	}

	if len(ents) == 0 {
		return nil
	}

	re := ents[rand.Intn(len(ents))]

	return d.Unlink(re.Name)
}

func actorReadFile(d *Directory) error {
	d, err := randomWalk(d, rand.Intn(6))
	if err != nil {
		return err
	}

	ents, err := d.List()
	if err != nil {
		return err
	}

	var files []string
	for _, e := range ents {
		if e.Type == int(TFile) {
			files = append(files, e.Name)
		}
	}

	if len(files) == 0 {
		return nil
	}

	fname := files[rand.Intn(len(files))]
	fsn, err := d.Child(fname)
	if err != nil {
		return err
	}

	fi, ok := fsn.(*File)
	if !ok {
		return errors.New("file wasnt a file, race?")
	}

	_, err = fi.Size()
	if err != nil {
		return err
	}

	_, err = ioutil.ReadAll(fi)
	if err != nil {
		return err
	}

	return fi.Close()
}

func testActor(rt *Root, iterations int, errs chan error) {
	d := rt.GetValue().(*Directory)
	for i := 0; i < iterations; i++ {
		switch rand.Intn(5) {
		case 0:
			if err := actorMkdir(d); err != nil {
				errs <- err
				return
			}
		case 1, 2:
			if err := actorMakeFile(d); err != nil {
				errs <- err
				return
			}
		case 3:
			continue
			// randomly deleting things
			// doesnt really give us any sort of useful test results.
			// you will never have this in a real environment where
			// you expect anything productive to happen...
			if err := actorRemoveFile(d); err != nil {
				errs <- err
				return
			}
		case 4:
			if err := actorReadFile(d); err != nil {
				errs <- err
				return
			}
		}
	}
	errs <- nil
}

func TestMfsStress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, rt := setupRoot(ctx, t)

	numroutines := 10

	errs := make(chan error)
	for i := 0; i < numroutines; i++ {
		go testActor(rt, 50, errs)
	}

	for i := 0; i < numroutines; i++ {
		err := <-errs
		if err != nil {
			t.Fatal(err)
		}
	}
}
