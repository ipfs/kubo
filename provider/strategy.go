package provider

import (
	"context"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	"gx/ipfs/QmdURv6Sbob8TVW2tFFve9vcEWrSUgwPqeqnXyvYhLrkyd/go-merkledag"
)

// anchor picker strategies

func NewAnchorAllStrategy(dag ipld.DAGService) AnchorStrategy {
	return func(ctx context.Context, cids chan cid.Cid, root cid.Cid) {
		cids <- root
		// TODO: Use schomatis' dag walker instead of this enumerate?
		merkledag.EnumerateChildren(ctx, merkledag.GetLinksWithDAG(dag), root, func(cid cid.Cid) bool {
			cids <- cid
			return true
		})
	}
}

// eligibility strategies

func NewEligibleOnlyOnceStrategy() EligibleStrategy {
	provided := cid.NewSet()
	return func(root cid.Cid) bool {
		eligible := !provided.Has(root)
		if eligible {
			provided.Add(root)
		}
		return eligible
	}
}