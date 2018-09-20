package options

import (
	"errors"
	"fmt"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	dag "gx/ipfs/QmcBoNcAP6qDjgRBew7yjvCqHq7p5jMstE44jPUBWBxzsV/go-merkledag"
)

type Layout int

const (
	BalancedLayout Layout = iota
	TrickleLeyout
)

type UnixfsAddSettings struct {
	CidVersion int
	MhType     uint64

	InlineLimit  int
	RawLeaves    bool
	RawLeavesSet bool

	Chunker string
	Layout  Layout

	Pin      bool
	OnlyHash bool
	Local    bool
}

type UnixfsAddOption func(*UnixfsAddSettings) error

func UnixfsAddOptions(opts ...UnixfsAddOption) (*UnixfsAddSettings, cid.Prefix, error) {
	options := &UnixfsAddSettings{
		CidVersion: -1,
		MhType:     mh.SHA2_256,

		InlineLimit:  0,
		RawLeaves:    false,
		RawLeavesSet: false,

		Chunker: "size-262144",
		Layout:  BalancedLayout,

		Pin:      false,
		OnlyHash: false,
		Local:    false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, cid.Prefix{}, err
		}
	}

	// (hash != "sha2-256") -> CIDv1
	if options.MhType != mh.SHA2_256 {
		switch options.CidVersion {
		case 0:
			return nil, cid.Prefix{}, errors.New("CIDv0 only supports sha2-256")
		case 1, -1:
			options.CidVersion = 1
		default:
			return nil, cid.Prefix{}, fmt.Errorf("unknown CID version: %d", options.CidVersion)
		}
	} else {
		if options.CidVersion < 0 {
			// Default to CIDv0
			options.CidVersion = 0
		}
	}

	// cidV1 -> raw blocks (by default)
	if options.CidVersion > 0 && !options.RawLeavesSet {
		options.RawLeaves = true
	}

	prefix, err := dag.PrefixForCidVersion(options.CidVersion)
	if err != nil {
		return nil, cid.Prefix{}, err
	}

	prefix.MhType = options.MhType
	prefix.MhLength = -1

	return options, prefix, nil
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

func (unixfsOpts) RawLeaves(enable bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.RawLeaves = enable
		settings.RawLeavesSet = true
		return nil
	}
}

func (unixfsOpts) InlineLimit(limit int) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.InlineLimit = limit
		return nil
	}
}

func (unixfsOpts) Chunker(chunker string) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Chunker = chunker
		return nil
	}
}

func (unixfsOpts) Layout(layout Layout) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Layout = layout
		return nil
	}
}

func (unixfsOpts) Pin(pin bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Pin = pin
		return nil
	}
}

func (unixfsOpts) HashOnly(hashOnly bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.OnlyHash = hashOnly
		return nil
	}
}

func (unixfsOpts) Local(local bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Local = local
		return nil
	}
}
