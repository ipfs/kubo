package options

import (
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
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

func UnixfsAddOptions(opts ...UnixfsAddOption) (*UnixfsAddSettings, error) {
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
