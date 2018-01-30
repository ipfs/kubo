package options

import (
	"gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
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

type BlockOptions struct{}

func (api *BlockOptions) WithFormat(codec string) BlockPutOption {
	return func(settings *BlockPutSettings) error {
		settings.Codec = codec
		return nil
	}
}

func (api *BlockOptions) WithHash(mhType uint64, mhLen int) BlockPutOption {
	return func(settings *BlockPutSettings) error {
		settings.MhType = mhType
		settings.MhLength = mhLen
		return nil
	}
}

func (api *BlockOptions) WithForce(force bool) BlockRmOption {
	return func(settings *BlockRmSettings) error {
		settings.Force = force
		return nil
	}
}
