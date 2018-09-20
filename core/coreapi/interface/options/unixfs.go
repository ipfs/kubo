package options

import (
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

type UnixfsAddSettings struct {
	CidVersion int
	MhType     uint64

	InlineLimit  int
	RawLeaves    bool
	RawLeavesSet bool
}

type UnixfsAddOption func(*UnixfsAddSettings) error

func UnixfsAddOptions(opts ...UnixfsAddOption) (*UnixfsAddSettings, error) {
	options := &UnixfsAddSettings{
		CidVersion: -1,
		MhType:     mh.SHA2_256,

		InlineLimit:  0,
		RawLeaves:    false,
		RawLeavesSet: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

type unixfsOpts struct{}

var Unixfs unixfsOpts

func (unixfsOpts) CidVersion(version int) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.CidVersion = version
		return nil
	}
}

func (unixfsOpts) Hash(mhtype uint64) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.MhType = mhtype
		return nil
	}
}
