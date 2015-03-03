package ipnsfs

import (
	"errors"
	"fmt"
	"os"

	dag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	ufspb "github.com/jbenet/go-ipfs/unixfs/pb"
)

type Directory struct {
	dserv     dag.DAGService
	parent    childCloser
	childDirs map[string]*Directory
	files     map[string]*file

	node *dag.Node
	name string
	ref  int
}

func NewDirectory(name string, node *dag.Node, parent childCloser, dserv dag.DAGService) *Directory {
	return &Directory{
		dserv:     dserv,
		name:      name,
		node:      node,
		parent:    parent,
		childDirs: make(map[string]*Directory),
		files:     make(map[string]*file),
	}
}

func (d *Directory) Open(tpath []string, mode int) (File, error) {
	log.Error("DIR OPEN:", tpath)
	if len(tpath) == 0 {
		return nil, ErrIsDirectory
	}
	if len(tpath) == 1 {
		fi, err := d.childFile(tpath[0])
		if err == nil {
			return fi.withMode(mode), nil
		}

		if mode|os.O_CREATE != 0 {
			fnode := new(dag.Node)
			fnode.Data = ft.FilePBData(nil, 0)
			nfi, err := NewFile(tpath[0], fnode, d, d.dserv)
			if err != nil {
				return nil, err
			}
			d.files[tpath[0]] = nfi
			return nfi.withMode(mode), nil
		}

		return nil, ErrNoSuch
	}

	dir, err := d.childDir(tpath[0])
	if err != nil {
		return nil, err
	}
	return dir.Open(tpath[1:], mode)
}

// consider combining into a single method...
type childCloser interface {
	closeChildFile(string) error
	closeChildDir(string) error
}

func (d *Directory) closeChildFile(name string) error {
	fi, ok := d.files[name]
	if !ok {
		return errors.New("no open child file by given name")
	}

	nnode, err := fi.mod.GetNode()
	if err != nil {
		log.Error("GET NODE")
		return err
	}

	_, err = d.dserv.Add(nnode)
	if err != nil {
		log.Error("ADD")
		return err
	}

	err = d.node.RemoveNodeLink(name)
	if err != nil && err != dag.ErrNotFound {
		log.Error("REMOVE")
		return err
	}

	err = d.node.AddNodeLink(name, nnode)
	if err != nil {
		log.Error("RE ADD")
		return err
	}

	return d.parent.closeChildDir(d.name)
}

func (d *Directory) closeChildDir(name string) error {
	panic("NYI")
}

func (d *Directory) childFile(name string) (*file, error) {
	fi, ok := d.files[name]
	if ok {
		return fi, nil
	}

	// search dag
	for _, lnk := range d.node.Links {
		if lnk.Name == name {
			nd, err := lnk.GetNode(d.dserv)
			if err != nil {
				return nil, err
			}
			i, err := ft.FromBytes(nd.Data)
			if err != nil {
				return nil, err
			}

			switch i.GetType() {
			case ufspb.Data_Directory:
				return nil, ErrIsDirectory
			case ufspb.Data_File:
				return NewFile(name, nd, d, d.dserv)
			case ufspb.Data_Metadata:
				panic("NOT YET IMPLEMENTED")
			default:
				panic("NO!")
			}
		}
	}
	return nil, ErrNoSuch
}

func (d *Directory) childDir(name string) (*Directory, error) {
	dir, ok := d.childDirs[name]
	if ok {
		return dir, nil
	}

	for _, lnk := range d.node.Links {
		if lnk.Name == name {
			nd, err := lnk.GetNode(d.dserv)
			if err != nil {
				return nil, err
			}
			i, err := ft.FromBytes(nd.Data)
			if err != nil {
				return nil, err
			}

			switch i.GetType() {
			case ufspb.Data_Directory:
				return NewDirectory(name, nd, d, d.dserv), nil
			case ufspb.Data_File:
				return nil, fmt.Errorf("%s is not a directory", name)
			case ufspb.Data_Metadata:
				panic("NOT YET IMPLEMENTED")
			default:
				panic("NO!")
			}
		}

	}

	return nil, ErrNoSuch
}
