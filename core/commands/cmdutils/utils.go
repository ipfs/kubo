package cmdutils

import (
	"fmt"

	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/ipfs/go-cid"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

const (
	AllowBigBlockOptionName = "allow-big-block"
	SoftBlockLimit          = 1024 * 1024 * 2 // https://github.com/web3-storage/web3.storage/pull/1269#issuecomment-1108834504
)

var AllowBigBlockOption cmds.Option

func init() {
	AllowBigBlockOption = cmds.BoolOption(AllowBigBlockOptionName, "Disable block size check and allow creation of blocks bigger than 2MiB. WARNING: such blocks won't be transferable over the standard bitswap.").WithDefault(false)
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

	// We do not allow producing blocks bigger than 2MiB to avoid errors
	// when transmitting them over BitSwap. The 2MiB constant is an
	// unenforced and undeclared rule of thumb hard-coded here.
	if size > SoftBlockLimit {
		return fmt.Errorf("produced block is over 2MiB: big blocks can't be exchanged with other peers. consider using UnixFS for automatic chunking of bigger files, or pass --allow-big-block to override")
	}
	return nil

}
