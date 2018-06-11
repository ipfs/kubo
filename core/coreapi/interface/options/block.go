package options

import (
	"gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

type BlockPutSettings struct {
	Codec    string
	MhType   uint64
	MhLength int
}

type BlockRmSettings struct {
	Force bool
}

type BlockPutOption func(*BlockPutSettings) error
type BlockRmOption func(*BlockRmSettings) error

func BlockPutOptions(opts ...BlockPutOption) (*BlockPutSettings, error) {
	options := &BlockPutSettings{
		Codec:    "v0",
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

func BlockRmOptions(opts ...BlockRmOption) (*BlockRmSettings, error) {
	options := &BlockRmSettings{
		Force: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type blockOpts struct{}

var Block blockOpts

// Format is an option for Block.Put which specifies the multicodec to use to
// serialize the object. Default is "v0"
func (blockOpts) Format(codec string) BlockPutOption {
	return func(settings *BlockPutSettings) error {
		settings.Codec = codec
		return nil
	}
}

// Hash is an option for Block.Put which specifies the multihash settings to use
// when hashing the object. Default is mh.SHA2_256 (0x12).
// If mhLen is set to -1, default length for the hash will be used
func (blockOpts) Hash(mhType uint64, mhLen int) BlockPutOption {
	return func(settings *BlockPutSettings) error {
		settings.MhType = mhType
		settings.MhLength = mhLen
		return nil
	}
}

// Force is an option for Block.Rm which, when set to true, will ignore
// non-existing blocks
func (blockOpts) Force(force bool) BlockRmOption {
	return func(settings *BlockRmSettings) error {
		settings.Force = force
		return nil
	}
}
