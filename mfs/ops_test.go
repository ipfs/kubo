package mfs

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/ipfs/go-ipfs/thirdparty/assert"

	ipld "gx/ipfs/QmUSyMZ8Vt4vTZr5HdDEgEfpwAXfQRuDdfCFTt7XBzhxpQ/go-ipld-format"
)

func TestGetNodeAndParent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, rt := setupRoot(ctx, t)
	p := "/a/b/c/d/e/f"
	err := Mkdir(rt, p, MkdirOpts{Mkparents: true, Flush: true})
	assert.Nil(err, t)

	nodeName, node, dir, err := getNodeAndParent(rt, p)
	assert.Nil(err, t)

	assert.True(nodeName == "f", t, fmt.Sprintf("expected node name: f, while real name is: %s", nodeName))
	assert.True(dir.name == "e", t, fmt.Sprintf("expected dir name: e, while real name is: %s", dir.name))

	fn, err := dir.Child(nodeName)
	assert.Nil(err, t)

	n, err := fn.GetNode()
	assert.Nil(err, t)

	assert.True(reflect.DeepEqual(n, node),
		t, fmt.Sprintf("expected node: %v, while real node is: %v", node, n))
}

func TestNodeType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, rt := setupRoot(ctx, t)
	p := "/a/b/c/d/e/f"
	err := Mkdir(rt, p, MkdirOpts{Mkparents: true, Flush: true})

	node, err := DirLookup(rt.GetDirectory(), p)
	assert.Nil(err, t)

	assert.True(node.Type() == TDir, t)

	n, err := node.GetNode()
	assert.Nil(err, t)

	p2 := "/a/b/c/d/e/f/"
	err = Mkdir(rt, p, MkdirOpts{Mkparents: true, Flush: true})

	node2, err := DirLookup(rt.GetDirectory(), p2)
	assert.Nil(err, t)

	assert.True(node2.Type() == TDir, t)

	n2, err := node2.GetNode()
	assert.Nil(err, t)

	assert.True(n.String() == n2.String(), t)
}

func TestMvDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ds, rt := setupRoot(ctx, t)

	dirs := []string{"/a", "/b", "/b/a", "/c", "/b/c/d", "/y", "/z"}
	for _, dir := range dirs {
		err := Mkdir(rt, dir, MkdirOpts{Mkparents: true, Flush: true})
		assert.Nil(err, t)
	}

	nd := getRandFile(t, ds, 1000)

	err := rt.GetDirectory().AddChild("x", nd)
	assert.Nil(err, t)

	err = Mv(rt, "/a", "/x")
	assert.True(reflect.DeepEqual(err, errMvDirToFile), t)

	err = Mv(rt, "/a", "/x/")
	assert.True(reflect.DeepEqual(err, errInvalidDirPath), t)

	err = Mv(rt, "/b", "/b/a")
	assert.True(reflect.DeepEqual(err, errMvParentDir), t)

	err = Mv(rt, "/c", "/b")
	assert.True(reflect.DeepEqual(err, ErrDirExists), t)

	err = Mv(rt, "/a", "/b")
	assert.Nil(err, t)
	checkDirExisted(rt.GetDirectory(), "/b/a", t)

	err = Mv(rt, "/y", "/b")
	assert.Nil(err, t)
	checkDirExisted(rt.GetDirectory(), "/b/y", t)

	err = Mv(rt, "/z/", "/b")
	assert.Nil(err, t)
	checkDirExisted(rt.GetDirectory(), "/b/z", t)
}

func TestMvFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ds, rt := setupRoot(ctx, t)

	dirs := []string{"/a", "/b", "/b/m/c", "/b/n"}
	for _, dir := range dirs {
		err := Mkdir(rt, dir, MkdirOpts{Mkparents: true, Flush: true})
		assert.Nil(err, t)
	}

	addFile("/", "x", rt, ds, t)
	addFile("/", "m", rt, ds, t)
	addFile("/", "n", rt, ds, t)
	addFile("/a", "y", rt, ds, t)
	addFile("/", "z", rt, ds, t)

	err := Mv(rt, "/x", "/a/y")
	assert.Nil(err, t)
	_, err = DirLookup(rt.GetDirectory(), "/a/y")
	assert.True(reflect.DeepEqual(err, os.ErrNotExist), t)
	checkFileExisted(rt.GetDirectory(), "/a/x", t)

	err = Mv(rt, "/z", "/b")
	assert.Nil(err, t)

	_, err = DirLookup(rt.GetDirectory(), "/z")
	assert.True(reflect.DeepEqual(err, os.ErrNotExist), t)
	checkFileExisted(rt.GetDirectory(), "/b/z", t)

	err = Mv(rt, "/m", "/b")
	assert.Err(err, t)

	err = Mv(rt, "/n", "/b")
	assert.Nil(err, t)
	_, err = DirLookup(rt.GetDirectory(), "/n")
	assert.True(reflect.DeepEqual(err, os.ErrNotExist), t)
	checkFileExisted(rt.GetDirectory(), "/b/n", t)
}

func checkFileExisted(dir *Directory, path string, t *testing.T) {
	node, err := DirLookup(dir, path)
	assert.Nil(err, t, fmt.Sprintf("lookup path: %s", path))
	assert.True(node.Type() == TFile, t, fmt.Sprintf("check file: %s", path))
}

func checkDirExisted(dir *Directory, path string, t *testing.T) {
	node, err := DirLookup(dir, path)
	assert.Nil(err, t, fmt.Sprintf("lookup path: %s", path))
	assert.True(node.Type() == TDir, t, fmt.Sprintf("check file: %s", path))
}

func addFile(parentDir, fileName string, r *Root, ds ipld.DAGService, t *testing.T) {
	dir, err := lookupDir(r, parentDir)
	assert.Nil(err, t, fmt.Sprintf("find dir: %s ", parentDir))
	nd := getRandFile(t, ds, 1000)
	err = dir.AddChild(fileName, nd)
	assert.Nil(err, t)
}
