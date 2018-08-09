package mfs

import (
	"errors"
	"fmt"
	"os"
	gopath "path"
	"strings"

	path "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path"

	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
)

var (
	errInvalidNodeType = errors.New("invalid argument")
	errMvDirToFile     = errors.New("can not move directory to file path")
	errMvParentDir     = errors.New("can not move parent directory to sub directory")
	errInvalidDirPath  = errors.New("path end with '/' is not a directory")
)

// Mv moves the file or directory at 'src' to 'dst'
func Mv(r *Root, src, dst string) error {
	if src == "/" {
		return errMvParentDir
	}

	src = strings.TrimRight(src, "/")
	if strings.HasPrefix(dst, src) {
		return errMvParentDir
	}

	srcNode, err := DirLookup(r.GetDirectory(), src)
	if err != nil {
		return err
	}

	if srcNode.Type() == TDir {
		return moveDir(r, src, dst)
	} else if srcNode.Type() == TFile {
		if src[len(src)-1] == '/' {
			return errInvalidDirPath
		}
		return moveFile(r, src, dst)
	} else {
		return ErrInvalidChild
	}
}

// moveDir moves the directory at 'src' to 'dst'
func moveDir(r *Root, src, dst string) error {
	src = strings.TrimRight(src, "/")
	srcDirName, srcNode, srcParDir, err := getNodeAndParent(r, src)
	if err != nil {
		return err
	}

	dstNode, err := DirLookup(r.GetDirectory(), dst)
	if err != nil {
		return err
	}

	if dstNode.Type() == TDir {
		dstDir, _ := dstNode.(*Directory)
		if n, err := dstDir.Child(srcDirName); err == nil { // contains srcDir
			empty, err := isEmptyNode(n)
			if err != nil {
				return err
			}
			if empty {
				if err = dstDir.Unlink(srcDirName); err != nil {
					return err
				}
			} else {
				return ErrDirExists
			}
		}

		if err = dstDir.AddChild(srcDirName, srcNode); err != nil {
			return err
		}
		return srcParDir.Unlink(srcDirName)
	} else if dstNode.Type() == TFile {
		if dst[len(dst)-1] == '/' {
			return errInvalidDirPath
		}
		return errMvDirToFile
	} else {
		return errInvalidNodeType
	}
}

// moveFile moves the file at 'src' to 'dst'
func moveFile(r *Root, src, dst string) error {
	srcFileName, srcNode, srcParDir, err := getNodeAndParent(r, src)
	if err != nil {
		return err
	}

	dstNode, err := DirLookup(r.GetDirectory(), dst)
	if err != nil {
		return err
	}

	if dstNode.Type() == TFile {
		if dst[len(dst)-1] == '/' {
			return errInvalidDirPath
		}
		// replace(src, dst)
		dstFileName, _, dstParDir, err := getNodeAndParent(r, dst)
		if err != nil {
			return err
		}

		err = dstParDir.Unlink(dstFileName)
		if err != nil {
			return err
		}

		err = dstParDir.AddChild(srcFileName, srcNode)
		if err != nil {
			return err
		}

		return srcParDir.Unlink(srcFileName)
	} else if dstNode.Type() == TDir {
		dstDir, _ := dstNode.(*Directory)
		if n, err := dstDir.Child(srcFileName); err == nil {
			// contains child with srcFileName
			empty, err := isEmptyNode(n)
			if err != nil {
				return err
			}

			if !empty && n.Type() == TDir {
				return ErrDirExists
			}

			if err = dstDir.Unlink(srcFileName); err != nil {
				return err
			}
		}

		err := dstDir.AddChild(srcFileName, srcNode)
		if err != nil {
			return err
		}

		return srcParDir.Unlink(srcFileName)
	} else {
		return errInvalidNodeType
	}
}

//getNodeAndParent find node and it's parent dir with path like: "/x/y/filename"
func getNodeAndParent(r *Root, path string) (string, ipld.Node, *Directory, error) {
	parentDirStr, filename := gopath.Split(path)

	parentDirObj, err := lookupDir(r, parentDirStr)
	if err != nil {
		return filename, nil, nil, err
	}

	fileObj, err := parentDirObj.Child(filename)
	if err != nil {
		return filename, nil, nil, err
	}

	nd, err := fileObj.GetNode()
	if err != nil {
		return filename, nil, nil, err
	}

	return filename, nd, parentDirObj, nil
}

func isEmptyNode(n FSNode) (bool, error) {
	in, err := n.GetNode()
	if err != nil {
		return false, err
	}

	return len(in.Links()) == 0, nil
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
func PutNode(r *Root, path string, nd ipld.Node) error {
	dirp, filename := gopath.Split(path)
	if filename == "" {
		return fmt.Errorf("cannot create file with empty name")
	}

	pdir, err := lookupDir(r, dirp)
	if err != nil {
		return err
	}

	return pdir.AddChild(filename, nd)
}

// MkdirOpts is used by Mkdir
type MkdirOpts struct {
	Mkparents  bool
	Flush      bool
	CidBuilder cid.Builder
}

// Mkdir creates a directory at 'path' under the directory 'd', creating
// intermediary directories as needed if 'mkparents' is set to true
func Mkdir(r *Root, pth string, opts MkdirOpts) error {
	if pth == "" {
		return fmt.Errorf("no path given to Mkdir")
	}
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
		if opts.Mkparents {
			return nil
		}
		return fmt.Errorf("cannot create directory '/': Already exists")
	}

	cur := r.GetDirectory()
	for i, d := range parts[:len(parts)-1] {
		fsn, err := cur.Child(d)
		if err == os.ErrNotExist && opts.Mkparents {
			mkd, err := cur.Mkdir(d)
			if err != nil {
				return err
			}
			if opts.CidBuilder != nil {
				mkd.SetCidBuilder(opts.CidBuilder)
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

	final, err := cur.Mkdir(parts[len(parts)-1])
	if err != nil {
		if !opts.Mkparents || err != os.ErrExist || final == nil {
			return err
		}
	}
	if opts.CidBuilder != nil {
		final.SetCidBuilder(opts.CidBuilder)
	}

	if opts.Flush {
		err := final.Flush()
		if err != nil {
			return err
		}
	}

	return nil
}

// Lookup extracts the root directory and performs a lookup under it.
// TODO: Now that the root is always a directory, can this function
// be collapsed with `DirLookup`? Or at least be made a method of `Root`?
func Lookup(r *Root, path string) (FSNode, error) {
	dir := r.GetDirectory()

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

func FlushPath(rt *Root, pth string) error {
	nd, err := Lookup(rt, pth)
	if err != nil {
		return err
	}

	err = nd.Flush()
	if err != nil {
		return err
	}

	rt.repub.WaitPub()
	return nil
}
