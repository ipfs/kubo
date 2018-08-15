// +build !nofuse

package readonly

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sync"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	coremock "github.com/ipfs/go-ipfs/core/mock"
	dag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	importer "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs/importer"
	uio "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs/io"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	fstest "gx/ipfs/QmSJBsmLP1XMjv8hxYg2rUMdPDB7YUpyBo9idjrJ6Cmq6F/fuse/fs/fstestutil"
	chunker "gx/ipfs/QmWbCAB5f3LDumj4ncz1UCHSiyXrXxkMxZB6Wv35xi4P8z/go-ipfs-chunker"
	ci "gx/ipfs/QmXG74iiKQnDstVQq9fPFQEB6JTNSWBbAWE1qsq6L4E5sR/go-testutil/ci"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
)

func maybeSkipFuseTests(t *testing.T) {
	if ci.NoFuse() {
		t.Skip("Skipping FUSE tests")
	}
}

func randObj(t *testing.T, nd *core.IpfsNode, size int64) (ipld.Node, []byte) {
	buf := make([]byte, size)
	u.NewTimeSeededRand().Read(buf)
	read := bytes.NewReader(buf)
	obj, err := importer.BuildTrickleDagFromReader(nd.DAG, chunker.DefaultSplitter(read))
	if err != nil {
		t.Fatal(err)
	}

	return obj, buf
}

func setupIpfsTest(t *testing.T, node *core.IpfsNode) (*core.IpfsNode, *fstest.Mount) {
	maybeSkipFuseTests(t)

	var err error
	if node == nil {
		node, err = coremock.NewMockNode()
		if err != nil {
			t.Fatal(err)
		}
	}

	fs := NewFileSystem(node)
	mnt, err := fstest.MountedT(t, fs, nil)
	if err != nil {
		t.Fatal(err)
	}

	return node, mnt
}

// Test writing an object and reading it back through fuse
func TestIpfsBasicRead(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	nd, mnt := setupIpfsTest(t, nil)
	defer mnt.Close()

	fi, data := randObj(t, nd, 10000)
	k := fi.Cid()
	fname := path.Join(mnt.Dir, k.String())
	rbuf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}
}

func getPaths(t *testing.T, ipfs *core.IpfsNode, name string, n *dag.ProtoNode) []string {
	if len(n.Links()) == 0 {
		return []string{name}
	}
	var out []string
	for _, lnk := range n.Links() {
		child, err := lnk.GetNode(ipfs.Context(), ipfs.DAG)
		if err != nil {
			t.Fatal(err)
		}

		childpb, ok := child.(*dag.ProtoNode)
		if !ok {
			t.Fatal(dag.ErrNotProtobuf)
		}

		sub := getPaths(t, ipfs, path.Join(name, lnk.Name), childpb)
		out = append(out, sub...)
	}
	return out
}

// Perform a large number of concurrent reads to stress the system
func TestIpfsStressRead(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	nd, mnt := setupIpfsTest(t, nil)
	defer mnt.Close()

	var nodes []ipld.Node
	var paths []string

	nobj := 50
	ndiriter := 50

	// Make a bunch of objects
	for i := 0; i < nobj; i++ {
		fi, _ := randObj(t, nd, rand.Int63n(50000))
		nodes = append(nodes, fi)
		paths = append(paths, fi.Cid().String())
	}

	// Now make a bunch of dirs
	for i := 0; i < ndiriter; i++ {
		db := uio.NewDirectory(nd.DAG)
		for j := 0; j < 1+rand.Intn(10); j++ {
			name := fmt.Sprintf("child%d", j)

			err := db.AddChild(nd.Context(), name, nodes[rand.Intn(len(nodes))])
			if err != nil {
				t.Fatal(err)
			}
		}
		newdir, err := db.GetNode()
		if err != nil {
			t.Fatal(err)
		}

		err = nd.DAG.Add(nd.Context(), newdir)
		if err != nil {
			t.Fatal(err)
		}

		nodes = append(nodes, newdir)
		npaths := getPaths(t, nd, newdir.Cid().String(), newdir.(*dag.ProtoNode))
		paths = append(paths, npaths...)
	}

	// Now read a bunch, concurrently
	wg := sync.WaitGroup{}
	errs := make(chan error)

	for s := 0; s < 4; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i < 2000; i++ {
				item := paths[rand.Intn(len(paths))]
				fname := path.Join(mnt.Dir, item)
				rbuf, err := ioutil.ReadFile(fname)
				if err != nil {
					errs <- err
				}

				read, err := coreunix.Cat(nd.Context(), nd, item)
				if err != nil {
					errs <- err
				}

				data, err := ioutil.ReadAll(read)
				if err != nil {
					errs <- err
				}

				if !bytes.Equal(rbuf, data) {
					errs <- errors.New("incorrect read")
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Test writing a file and reading it back
func TestIpfsBasicDirRead(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	nd, mnt := setupIpfsTest(t, nil)
	defer mnt.Close()

	// Make a 'file'
	fi, data := randObj(t, nd, 10000)

	// Make a directory and put that file in it
	db := uio.NewDirectory(nd.DAG)
	err := db.AddChild(nd.Context(), "actual", fi)
	if err != nil {
		t.Fatal(err)
	}

	d1nd, err := db.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	err = nd.DAG.Add(nd.Context(), d1nd)
	if err != nil {
		t.Fatal(err)
	}

	dirname := path.Join(mnt.Dir, d1nd.Cid().String())
	fname := path.Join(dirname, "actual")
	rbuf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	dirents, err := ioutil.ReadDir(dirname)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirents) != 1 {
		t.Fatal("Bad directory entry count")
	}
	if dirents[0].Name() != "actual" {
		t.Fatal("Bad directory entry")
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}
}

// Test to make sure the filesystem reports file sizes correctly
func TestFileSizeReporting(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	nd, mnt := setupIpfsTest(t, nil)
	defer mnt.Close()

	fi, data := randObj(t, nd, 10000)
	k := fi.Cid()

	fname := path.Join(mnt.Dir, k.String())

	finfo, err := os.Stat(fname)
	if err != nil {
		t.Fatal(err)
	}

	if finfo.Size() != int64(len(data)) {
		t.Fatal("Read incorrect size from stat!")
	}
}
