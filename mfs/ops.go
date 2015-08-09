package mfs

import (
	"errors"
	"fmt"
	"os"
	gopath "path"
	"strings"

	dag "github.com/ipfs/go-ipfs/merkledag"
)

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

func PutNodeUnderRoot(root *Root, ipath string, nd *dag.Node) error {
	dir, ok := root.GetValue().(*Directory)
	if !ok {
		return errors.New("root did not point to directory")
	}
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
