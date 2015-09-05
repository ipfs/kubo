package dagutils

import (
	"bytes"
	"fmt"
	"path"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
)

const (
	Add = iota
	Remove
	Mod
)

type Change struct {
	Type   int
	Path   string
	Before key.Key
	After  key.Key
}

func (c *Change) String() string {
	switch c.Type {
	case Add:
		return fmt.Sprintf("Added %s at %s", c.After.B58String()[:6], c.Path)
	case Remove:
		return fmt.Sprintf("Removed %s from %s", c.Before.B58String()[:6], c.Path)
	case Mod:
		return fmt.Sprintf("Changed %s to %s at %s", c.Before.B58String()[:6], c.After.B58String()[:6], c.Path)
	default:
		panic("nope")
	}
}

func ApplyChange(ctx context.Context, ds dag.DAGService, nd *dag.Node, cs []*Change) (*dag.Node, error) {
	e := NewDagEditor(ds, nd)
	for _, c := range cs {
		switch c.Type {
		case Add:
			child, err := ds.Get(ctx, c.After)
			if err != nil {
				return nil, err
			}
			err = e.InsertNodeAtPath(ctx, c.Path, child, nil)
			if err != nil {
				return nil, err
			}

		case Remove:
			err := e.RmLink(ctx, c.Path)
			if err != nil {
				return nil, err
			}

		case Mod:
			err := e.RmLink(ctx, c.Path)
			if err != nil {
				return nil, err
			}
			child, err := ds.Get(ctx, c.After)
			if err != nil {
				return nil, err
			}
			err = e.InsertNodeAtPath(ctx, c.Path, child, nil)
			if err != nil {
				return nil, err
			}
		}
	}
	return e.GetNode(), nil
}

func Diff(ctx context.Context, ds dag.DAGService, a, b *dag.Node) []*Change {
	if len(a.Links) == 0 && len(b.Links) == 0 {
		ak, _ := a.Key()
		bk, _ := b.Key()
		return []*Change{
			&Change{
				Type:   Mod,
				Before: ak,
				After:  bk,
			},
		}
	}

	var out []*Change
	clean_a := a.Copy()
	clean_b := b.Copy()

	// strip out unchanged stuff
	for _, lnk := range a.Links {
		l, err := b.GetNodeLink(lnk.Name)
		if err == nil {
			if bytes.Equal(l.Hash, lnk.Hash) {
				// no change... ignore it
			} else {
				anode, _ := lnk.GetNode(ctx, ds)
				bnode, _ := l.GetNode(ctx, ds)
				sub := Diff(ctx, ds, anode, bnode)

				for _, subc := range sub {
					subc.Path = path.Join(lnk.Name, subc.Path)
					out = append(out, subc)
				}
			}
			clean_a.RemoveNodeLink(l.Name)
			clean_b.RemoveNodeLink(l.Name)
		}
	}

	for _, lnk := range clean_a.Links {
		out = append(out, &Change{
			Type:   Remove,
			Path:   lnk.Name,
			Before: key.Key(lnk.Hash),
		})
	}
	for _, lnk := range clean_b.Links {
		out = append(out, &Change{
			Type:  Add,
			Path:  lnk.Name,
			After: key.Key(lnk.Hash),
		})
	}

	return out
}

type Conflict struct {
	A *Change
	B *Change
}

func MergeDiffs(a, b []*Change) ([]*Change, []Conflict) {
	var out []*Change
	var conflicts []Conflict
	paths := make(map[string]*Change)
	for _, c := range a {
		paths[c.Path] = c
	}

	for _, c := range b {
		if ca, ok := paths[c.Path]; ok {
			conflicts = append(conflicts, Conflict{
				A: ca,
				B: c,
			})
		} else {
			out = append(out, c)
		}
	}
	for _, c := range paths {
		out = append(out, c)
	}
	return out, conflicts
}
