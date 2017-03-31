package dagutils

import (
	"context"
	"fmt"

	mdag "github.com/ipfs/go-ipfs/merkledag"

	node "github.com/ipfs/go-ipld-node"
	cid "gx/ipfs/QmYhQaCYEcaPPjxJX7YcPcVKkQfRy6sJ7B3XmGFk82XYdQ/go-cid"
)

// DiffEnumerate fetches every object in the graph pointed to by 'to' that is
// not in 'from'. This can be used to more efficiently fetch a graph if you can
// guarantee you already have the entirety of 'from'
func DiffEnumerate(ctx context.Context, dserv node.NodeGetter, from, to *cid.Cid) error {
	fnd, err := dserv.Get(ctx, from)
	if err != nil {
		return fmt.Errorf("get %s: %s", from, err)
	}

	tnd, err := dserv.Get(ctx, to)
	if err != nil {
		return fmt.Errorf("get %s: %s", to, err)
	}

	diff := getLinkDiff(fnd, tnd)

	sset := cid.NewSet()
	for _, c := range diff {
		if c.a == nil {
			err := mdag.EnumerateChildrenAsync(ctx, mdag.GetLinksDirect(dserv), c.b, sset.Visit)
			if err != nil {
				return err
			}
		} else {
			err := DiffEnumerate(ctx, dserv, c.a, c.b)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type diffpair struct {
	a, b *cid.Cid
}

func getLinkDiff(a, b node.Node) []diffpair {
	have := make(map[string]*node.Link)
	names := make(map[string]*node.Link)
	for _, l := range a.Links() {
		have[l.Cid.KeyString()] = l
		names[l.Name] = l
	}

	var out []diffpair

	for _, l := range b.Links() {
		if have[l.Cid.KeyString()] != nil {
			continue
		}

		match, ok := names[l.Name]
		if !ok {
			out = append(out, diffpair{b: l.Cid})
			continue
		}

		out = append(out, diffpair{a: match.Cid, b: l.Cid})
	}
	return out
}
