package cmdutils

import (
	"fmt"

	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	coreiface "github.com/ipfs/kubo/core/coreiface"
)

const (
	AllowBigBlockOptionName = "allow-big-block"
	SoftBlockLimit          = 1024 * 1024 // https://github.com/ipfs/kubo/issues/7421#issuecomment-910833499
)

var AllowBigBlockOption cmds.Option

func init() {
	AllowBigBlockOption = cmds.BoolOption(AllowBigBlockOptionName, "Disable block size check and allow creation of blocks bigger than 1MiB. WARNING: such blocks won't be transferable over the standard bitswap.").WithDefault(false)
}

func CheckCIDSize(req *cmds.Request, c cid.Cid, dagAPI coreiface.APIDagService) error {
	n, err := dagAPI.Get(req.Context, c)
	if err != nil {
		return fmt.Errorf("CheckCIDSize: getting dag: %w", err)
	}

	nodeSize, err := n.Size()
	if err != nil {
		return fmt.Errorf("CheckCIDSize: getting node size: %w", err)
	}

	return CheckBlockSize(req, nodeSize)
}

func CheckBlockSize(req *cmds.Request, size uint64) error {
	allowAnyBlockSize, _ := req.Options[AllowBigBlockOptionName].(bool)
	if allowAnyBlockSize {
		return nil
	}

	// We do not allow producing blocks bigger than 1 MiB to avoid errors
	// when transmitting them over BitSwap. The 1 MiB constant is an
	// unenforced and undeclared rule of thumb hard-coded here.
	if size > SoftBlockLimit {
		return fmt.Errorf("produced block is over 1MiB: big blocks can't be exchanged with other peers. consider using UnixFS for automatic chunking of bigger files, or pass --allow-big-block to override")
	}
	return nil
}

// PathOrCidPath returns a path.Path built from the argument. It keeps the old
// behaviour by building a path from a CID string.
func PathOrCidPath(str string) (path.Path, error) {
	p, err := path.NewPath(str)
	if err == nil {
		return p, nil
	}

	if p, err := path.NewPath("/ipfs/" + str); err == nil {
		return p, nil
	}

	// Send back original err.
	return nil, err
}
