package options

import (
	"math"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

type DagPutSettings struct {
	InputEnc string
	Codec    uint64
	MhType   uint64
	MhLength int
}

type DagTreeSettings struct {
	Depth int
}

type DagPutOption func(*DagPutSettings) error
type DagTreeOption func(*DagTreeSettings) error

func DagPutOptions(opts ...DagPutOption) (*DagPutSettings, error) {
	options := &DagPutSettings{
		InputEnc: "json",
		Codec:    cid.DagCBOR,
		MhType:   math.MaxUint64,
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

func DagTreeOptions(opts ...DagTreeOption) (*DagTreeSettings, error) {
	options := &DagTreeSettings{
		Depth: -1,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type DagOptions struct{}

func (api *DagOptions) WithInputEnc(enc string) DagPutOption {
	return func(settings *DagPutSettings) error {
		settings.InputEnc = enc
		return nil
	}
}

func (api *DagOptions) WithCodec(codec uint64) DagPutOption {
	return func(settings *DagPutSettings) error {
		settings.Codec = codec
		return nil
	}
}

func (api *DagOptions) WithHash(mhType uint64, mhLen int) DagPutOption {
	return func(settings *DagPutSettings) error {
		settings.MhType = mhType
		settings.MhLength = mhLen
		return nil
	}
}

func (api *DagOptions) WithDepth(depth int) DagTreeOption {
	return func(settings *DagTreeSettings) error {
		settings.Depth = depth
		return nil
	}
}
