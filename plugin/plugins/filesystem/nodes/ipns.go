package fsnodes

import (
	"context"

	"github.com/hugelgupf/p9/p9"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var _ p9.File = (*IPNS)(nil)
var _ WalkRef = (*IPNS)(nil)

// IPNS exposes the IPNS API over a p9.File interface
// Walk does not expect a namespace, only its path argument
// e.g. `ipfs.Walk([]string("Qm...", "subdir")` not `ipfs.Walk([]string("ipns", "Qm...", "subdir")`
type IPNS = IPFS

func IPNSAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	id := &IPNS{IPFSBase: newIPFSBase(ctx, "/ipns", core, ops...)}
	id.Qid.Type = p9.TypeDir
	id.meta.Mode, id.metaMask.Mode = p9.ModeDirectory|IRXA, true
	return id
}
