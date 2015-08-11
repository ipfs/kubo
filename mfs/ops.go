package mfs

import (
	"fmt"
	"os"
	gopath "path"
	"strings"

	dag "github.com/ipfs/go-ipfs/merkledag"
)

// Mv moves the file or directory at 'src' to 'dst'
func Mv(rootd *Directory, src, dst string) error {
	srcDir, srcFname := gopath.Split(src)

	srcObj, err := DirLookup(rootd, src)
	if err != nil {
		return err
	}

	var dstDirStr string
	var filename string
	if dst[len(dst)-1] == '/' {
		dstDirStr = dst
		filename = srcFname
	} else {
		dstDirStr, filename = gopath.Split(dst)
	}

	dstDiri, err := DirLookup(rootd, dstDirStr)
	if err != nil {
		return err
	}

	dstDir := dstDiri.(*Directory)
	nd, err := srcObj.GetNode()
	if err != nil {
		return err
	}

	err = dstDir.AddChild(filename, nd)
	if err != nil {
		return err
	}

	srcDirObji, err := DirLookup(rootd, srcDir)
	if err != nil {
		return err
	}

	srcDirObj := srcDirObji.(*Directory)
	err = srcDirObj.Unlink(srcFname)
	if err != nil {
		return err
	}

	return nil
}

// PutNodeUnderRoot inserts 'nd' at 'ipath' under the given directory dir
func PutNodeUnderDir(dir *Directory, ipath string, nd *dag.Node) error {
	dirp, filename := gopath.Split(ipath)

	parent, err := DirLookup(dir, dirp)
	if err != nil {
		return fmt.Errorf("lookup '%s' failed: %s", dirp, err)
	}

	pdir, ok := parent.(*Directory)
	if !ok {
		return fmt.Errorf("%s did not point to directory", dirp)
	}

	return pdir.AddChild(filename, nd)
}

// Mkdir creates a directory at 'path' under the directory 'd', creating
// intermediary directories as needed if 'parents' is set to true
func Mkdir(rootd *Directory, path string, parents bool) error {
	parts := strings.Split(path, "/")
	if parts[0] == "" {
		parts = parts[1:]
	}

	cur := rootd
	for i, d := range parts[:len(parts)-1] {
		fsn, err := cur.Child(d)
		if err != nil {
			if err == os.ErrNotExist && parents {
				mkd, err := cur.Mkdir(d)
				if err != nil {
					return err
				}
				fsn = mkd
			}
		}

		next, ok := fsn.(*Directory)
		if !ok {
			return fmt.Errorf("%s was not a directory", strings.Join(parts[:i], "/"))
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

// DirLookup will look up a file or directory at the given path
// under the directory 'd'
func DirLookup(d *Directory, path string) (FSNode, error) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 1 && parts[0] == "" {
		return d, nil
	}

	var cur FSNode
	cur = d
	for i, p := range parts {
		chdir, ok := cur.(*Directory)
		if !ok {
			return nil, fmt.Errorf("cannot access %s: Not a directory", strings.Join(parts[:i+1], "/"))
		}

		child, err := chdir.Child(p)
		if err != nil {
			return nil, err
		}

		cur = child
	}
	return cur, nil
}
