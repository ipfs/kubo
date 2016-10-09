package dagutils

import (
	"bytes"
	"fmt"
	"path"

	dag "github.com/ipfs/go-ipfs/merkledag"

	context "context"
	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
)

const (
	Add = iota
	Remove
	Mod
)

type Change struct {
	Type   int
	Path   string
	Before *cid.Cid
	After  *cid.Cid
}

func (c *Change) String() string {
	switch c.Type {
	case Add:
		return fmt.Sprintf("Added %s at %s", c.After.String(), c.Path)
	case Remove:
		return fmt.Sprintf("Removed %s from %s", c.Before.String(), c.Path)
	case Mod:
		return fmt.Sprintf("Changed %s to %s at %s", c.Before.String(), c.After.String(), c.Path)
	default:
		panic("nope")
	}
}

func ApplyChange(ctx context.Context, ds dag.DAGService, nd *dag.Node, cs []*Change) (*dag.Node, error) {
	e := NewDagEditor(nd, ds)
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

	return e.Finalize(ds)
}

func Diff(ctx context.Context, ds dag.DAGService, a, b *dag.Node) ([]*Change, error) {
	if len(a.Links) == 0 && len(b.Links) == 0 {
		return []*Change{
			&Change{
				Type:   Mod,
				Before: a.Cid(),
				After:  b.Cid(),
			},
		}, nil
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
				anode, err := lnk.GetNode(ctx, ds)
				if err != nil {
					return nil, err
				}

				bnode, err := l.GetNode(ctx, ds)
				if err != nil {
					return nil, err
				}

				sub, err := Diff(ctx, ds, anode, bnode)
				if err != nil {
					return nil, err
				}

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
			Before: cid.NewCidV0(lnk.Hash),
		})
	}
	for _, lnk := range clean_b.Links {
		out = append(out, &Change{
			Type:  Add,
			Path:  lnk.Name,
			After: cid.NewCidV0(lnk.Hash),
		})
	}

	return out, nil
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
