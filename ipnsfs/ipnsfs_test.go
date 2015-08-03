package ipnsfs

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
	blockserv, err := bserv.New(bs, offline.Exchange(bs))
	if err != nil {
		t.Fatal(err)
	}
	dserv := dag.NewDAGService(blockserv)
	mpin := pin.NewPinner(db, dserv).GetManual()
	return dagservAndPinner{
		ds: dserv,
		mp: mpin,
	}
}

func getRandFile(t *testing.T, ds dag.DAGService, size int64) *dag.Node {
	r := io.LimitReader(u.NewTimeSeededRand(), size)
	nd, err := importer.BuildDagFromReader(r, ds, chunk.DefaultSplitter, nil)
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

func TestBasic(t *testing.T) {
	dsp := getDagservAndPinner(t)

	ctx := context.TODO()

	fs, err := NewFilesystem(ctx, dsp.ds, dsp.mp)
	if err != nil {
		t.Fatal(err)
	}

	root := &dag.Node{Data: ft.FolderPBData()}
	rt, err := fs.NewRoot("test", root, func(k key.Key) error {
		fmt.Println("PUBLISHED: ", k)
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	rootdir := rt.GetValue().(*Directory)

	// test making a basic dir
	_, err = rootdir.Mkdir("a")
	if err != nil {
		t.Fatal(err)
	}

	d := mkdirP(t, rootdir, "a/b/c/d/e/f/g")

	fi := getRandFile(t, dsp.ds, 1000)

	err = d.AddChild("afile", fi)
	if err != nil {
		t.Fatal(err)
	}

	err = assertFileAtPath(dsp.ds, rootdir, fi, "a/b/c/d/e/f/g/afile")
	if err != nil {
		t.Fatal(err)
	}
}
