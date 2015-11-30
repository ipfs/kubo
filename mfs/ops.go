package mfs

import (
	"errors"
	"fmt"
	"os"
	gopath "path"
	"strings"

	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
)

// Mv moves the file or directory at 'src' to 'dst'
func Mv(r *Root, src, dst string) error {
	srcDir, srcFname := gopath.Split(src)

	var dstDirStr string
	var filename string
	if dst[len(dst)-1] == '/' {
		dstDirStr = dst
		filename = srcFname
	} else {
		dstDirStr, filename = gopath.Split(dst)
	}

	// get parent directories of both src and dest first
	dstDir, err := lookupDir(r, dstDirStr)
	if err != nil {
		return err
	}

	srcDirObj, err := lookupDir(r, srcDir)
	if err != nil {
		return err
	}

	srcObj, err := srcDirObj.Child(srcFname)
	if err != nil {
		return err
	}

	nd, err := srcObj.GetNode()
	if err != nil {
		return err
	}

	fsn, err := dstDir.Child(filename)
	if err == nil {
		switch n := fsn.(type) {
		case *File:
			_ = dstDir.Unlink(filename)
		case *Directory:
			dstDir = n
		default:
			return fmt.Errorf("unexpected type at path: %s", dst)
		}
	} else if err != os.ErrNotExist {
		return err
	}

	err = dstDir.AddChild(filename, nd)
	if err != nil {
		return err
	}

	err = srcDirObj.Unlink(srcFname)
	if err != nil {
		return err
	}

	return nil
}

func lookupDir(r *Root, path string) (*Directory, error) {
	di, err := Lookup(r, path)
	if err != nil {
		return nil, err
	}

	d, ok := di.(*Directory)
	if !ok {
		return nil, fmt.Errorf("%s is not a directory", path)
	}

	return d, nil
}

// PutNode inserts 'nd' at 'path' in the given mfs
func PutNode(r *Root, path string, nd *dag.Node) error {
	dirp, filename := gopath.Split(path)

	pdir, err := lookupDir(r, dirp)
	if err != nil {
		return err
	}

	return pdir.AddChild(filename, nd)
}

// Mkdir creates a directory at 'path' under the directory 'd', creating
// intermediary directories as needed if 'parents' is set to true
func Mkdir(r *Root, pth string, parents bool) error {
	parts := path.SplitList(pth)
	if parts[0] == "" {
		parts = parts[1:]
	}

	// allow 'mkdir /a/b/c/' to create c
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	if len(parts) == 0 {
		// this will only happen on 'mkdir /'
		return fmt.Errorf("cannot mkdir '%s'", pth)
	}

	cur := r.GetValue().(*Directory)
	for i, d := range parts[:len(parts)-1] {
		fsn, err := cur.Child(d)
		if err == os.ErrNotExist && parents {
			mkd, err := cur.Mkdir(d)
			if err != nil {
				return err
			}
			fsn = mkd
		} else if err != nil {
			return err
		}

		next, ok := fsn.(*Directory)
		if !ok {
			return fmt.Errorf("%s was not a directory", path.Join(parts[:i]))
		}
		cur = next
	}

	_, err := cur.Mkdir(parts[len(parts)-1])
	if err != nil {
		if !parents || err != os.ErrExist {
			return err
		}
	}

	return nil
}

func Lookup(r *Root, path string) (FSNode, error) {
	dir, ok := r.GetValue().(*Directory)
	if !ok {
		return nil, errors.New("root was not a directory")
	}

	return DirLookup(dir, path)
}

// DirLookup will look up a file or directory at the given path
// under the directory 'd'
func DirLookup(d *Directory, pth string) (FSNode, error) {
	pth = strings.Trim(pth, "/")
	parts := path.SplitList(pth)
	if len(parts) == 1 && parts[0] == "" {
		return d, nil
	}

	var cur FSNode
	cur = d
	for i, p := range parts {
		chdir, ok := cur.(*Directory)
		if !ok {
			return nil, fmt.Errorf("cannot access %s: Not a directory", path.Join(parts[:i+1]))
		}

		child, err := chdir.Child(p)
		if err != nil {
			return nil, err
		}

		cur = child
	}
	return cur, nil
}
