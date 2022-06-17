package corehttp

import (
	"fmt"
	"github.com/ipfs/go-cid"
	"net/http"
	"time"

	"go.uber.org/zap"

	ipath "github.com/ipfs/interface-go-ipfs-core/path"

	dagpb "github.com/ipld/go-codec-dagpb"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/ipld/go-ipld-prime/traversal/selector"
)

func (i *gatewayHandler) serveIPLD(w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, selectorNode ipld.Node, selectorCID cid.Cid, begin time.Time, logger *zap.SugaredLogger) {
	if resolvedPath.Remainder() != "" {
		http.Error(w, "serving ipld cannot handle path remainders", http.StatusInternalServerError)
		return
	}

	nsc := func(lnk ipld.Link, lctx ipld.LinkContext) (ipld.NodePrototype, error) {
		// We can decode all nodes into basicnode's Any, except for
		// dagpb nodes, which must explicitly use the PBNode prototype.
		if lnk, ok := lnk.(cidlink.Link); ok && lnk.Cid.Prefix().Codec == 0x70 {
			return dagpb.Type.PBNode, nil
		}
		return basicnode.Prototype.Any, nil
	}

	compiledSelector, err := selector.CompileSelector(selectorNode)
	if err != nil {
		webError(w, "could not compile selector", err, http.StatusInternalServerError)
		return
	}

	lnk := cidlink.Link{Cid: resolvedPath.Cid()}
	ns, _ := nsc(lnk, ipld.LinkContext{}) // nsc won't error

	type HasLinksystem interface {
		LinkSystem() ipld.LinkSystem
	}

	lsHaver, ok := i.api.(HasLinksystem)
	if !ok {
		webError(w, "could not find linksystem", err, http.StatusInternalServerError)
		return
	}
	lsys := lsHaver.LinkSystem()

	nd, err := lsys.Load(ipld.LinkContext{Ctx: r.Context()}, lnk, ns)
	if err != nil {
		webError(w, "could not load root", err, http.StatusInternalServerError)
		return
	}

	prog := traversal.Progress{
		Cfg: &traversal.Config{
			Ctx:                            r.Context(),
			LinkSystem:                     lsys,
			LinkTargetNodePrototypeChooser: nsc,
			LinkVisitOnlyOnce:              true,
		},
	}

	var latestMatchedNode ipld.Node

	err = prog.WalkAdv(nd, compiledSelector, func(progress traversal.Progress, node datamodel.Node, reason traversal.VisitReason) error {
		if reason == traversal.VisitReason_SelectionMatch {
			if latestMatchedNode == nil {
				latestMatchedNode = node
			} else {
				return fmt.Errorf("can only use selectors that match a single node")
			}
		}
		return nil
	})
	if err != nil {
		webError(w, "could not execute selector", err, http.StatusInternalServerError)
		return
	}

	if latestMatchedNode == nil {
		webError(w, "selector did not match anything", err, http.StatusInternalServerError)
		return
	}

	lbnNode, ok := latestMatchedNode.(datamodel.LargeBytesNode)
	if !ok {
		webError(w, "matched node was not bytes", err, http.StatusInternalServerError)
		return
	}
	if data, err := lbnNode.AsLargeBytes(); err != nil {
		webError(w, "matched node was not bytes", err, http.StatusInternalServerError)
		return
	} else {
		modtime := addCacheControlHeaders(w, r, contentPath, resolvedPath.Cid())
		name := resolvedPath.Cid().String() + "-" + selectorCID.String()
		http.ServeContent(w, r, name, modtime, data)
	}
}
