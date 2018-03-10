package iface

import (
	"context"
	"io"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

// BlockStat contains information about a block
type BlockStat interface {
	// Size is the size of a block
	Size() int

	// Path returns path to the block
	Path() Path
}

// BlockAPI specifies the interface to the block layer
type BlockAPI interface {
	// Put imports raw block data, hashing it using specified settings.
	Put(context.Context, io.Reader, ...options.BlockPutOption) (Path, error)

	// WithFormat is an option for Put which specifies the multicodec to use to
	// serialize the object. Default is "v0"
	WithFormat(codec string) options.BlockPutOption

	// WithHash is an option for Put which specifies the multihash settings to use
	// when hashing the object. Default is mh.SHA2_256 (0x12).
	// If mhLen is set to -1, default length for the hash will be used
	WithHash(mhType uint64, mhLen int) options.BlockPutOption

	// Get attempts to resolve the path and return a reader for data in the block
	Get(context.Context, Path) (io.Reader, error)

	// Rm removes the block specified by the path from local blockstore.
	// By default an error will be returned if the block can't be found locally.
	//
	// NOTE: If the specified block is pinned it won't be removed and no error
	// will be returned
	Rm(context.Context, Path, ...options.BlockRmOption) error

	// WithForce is an option for Rm which, when set to true, will ignore
	// non-existing blocks
	WithForce(force bool) options.BlockRmOption

	// Stat returns information on
	Stat(context.Context, Path) (BlockStat, error)
}
