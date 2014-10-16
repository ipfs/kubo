package pin

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	"github.com/jbenet/go-ipfs/blocks/set"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/util"
)

type Pinner interface {
	Pin(*mdag.Node, bool) error
	Unpin(util.Key, bool) error
}

type pinner struct {
	recursePin set.BlockSet
	directPin  set.BlockSet
	indirPin   set.BlockSet
	dserv      *mdag.DAGService
}

func NewPinner(dstore ds.Datastore, serv *mdag.DAGService) Pinner {
	return &pinner{
		recursePin: set.NewSimpleBlockSet(),
		directPin:  set.NewSimpleBlockSet(),
		indirPin:   NewRefCountBlockSet(),
		dserv:      serv,
	}
}

func (p *pinner) Pin(node *mdag.Node, recurse bool) error {
	k, err := node.Key()
	if err != nil {
		return err
	}

	if recurse {
		if p.recursePin.HasKey(k) {
			return nil
		}

		p.recursePin.AddBlock(k)

		err := p.pinLinks(node)
		if err != nil {
			return err
		}
	} else {
		p.directPin.AddBlock(k)
	}
	return nil
}

func (p *pinner) Unpin(k util.Key, recurse bool) error {
	panic("not yet implemented!")
	return nil
}

func (p *pinner) pinIndirectRecurse(node *mdag.Node) error {
	k, err := node.Key()
	if err != nil {
		return err
	}

	p.indirPin.AddBlock(k)
	return p.pinLinks(node)
}

func (p *pinner) pinLinks(node *mdag.Node) error {
	for _, l := range node.Links {
		subnode, err := l.GetNode(p.dserv)
		if err != nil {
			// TODO: Maybe just log and continue?
			return err
		}
		err = p.pinIndirectRecurse(subnode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *pinner) IsPinned(key util.Key) bool {
	return p.recursePin.HasKey(key) ||
		p.directPin.HasKey(key) ||
		p.indirPin.HasKey(key)
}
