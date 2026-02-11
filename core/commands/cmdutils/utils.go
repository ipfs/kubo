package cmdutils

import (
	"fmt"
	"slices"

	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	AllowBigBlockOptionName = "allow-big-block"
	// SoftBlockLimit is the maximum block size for bitswap transfer.
	// If this value changes, update the "2MiB" strings in error messages below.
	SoftBlockLimit  = 2 * 1024 * 1024 // https://specs.ipfs.tech/bitswap-protocol/#block-sizes
	MaxPinNameBytes = 255             // Maximum number of bytes allowed for a pin name
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

	// Block size is limited to SoftBlockLimit (2MiB) as defined in the bitswap spec.
	// https://specs.ipfs.tech/bitswap-protocol/#block-sizes
	if size > SoftBlockLimit {
		return fmt.Errorf("produced block is over 2MiB: big blocks can't be exchanged with other peers. consider using UnixFS for automatic chunking of bigger files, or pass --allow-big-block to override")
	}
	return nil
}

// ValidatePinName validates that a pin name does not exceed the maximum allowed byte length.
// Returns an error if the name exceeds MaxPinNameBytes (255 bytes).
func ValidatePinName(name string) error {
	if name == "" {
		// Empty names are allowed
		return nil
	}

	nameBytes := len([]byte(name))
	if nameBytes > MaxPinNameBytes {
		return fmt.Errorf("pin name is %d bytes (max %d bytes)", nameBytes, MaxPinNameBytes)
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

	// Save the original error before attempting fallback
	originalErr := err

	if p, err := path.NewPath("/ipfs/" + str); err == nil {
		return p, nil
	}

	// Send back original err.
	return nil, originalErr
}

// CloneAddrInfo returns a copy of the AddrInfo with a cloned Addrs slice.
// This prevents data races if the sender reuses the backing array.
// See: https://github.com/ipfs/kubo/issues/11116
func CloneAddrInfo(ai peer.AddrInfo) peer.AddrInfo {
	return peer.AddrInfo{
		ID:    ai.ID,
		Addrs: slices.Clone(ai.Addrs),
	}
}
