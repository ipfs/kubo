package options

import (
	"errors"
	"fmt"
	"os"
	"time"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	"github.com/ipfs/boxo/ipld/unixfs/io"
	cid "github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

type Layout int

const (
	BalancedLayout Layout = iota
	TrickleLayout
)

type UnixfsAddSettings struct {
	CidVersion int
	MhType     uint64

	Inline               bool
	InlineLimit          int
	RawLeaves            bool
	RawLeavesSet         bool
	MaxFileLinks         int
	MaxFileLinksSet      bool
	MaxDirectoryLinks    int
	MaxDirectoryLinksSet bool
	MaxHAMTFanout        int
	MaxHAMTFanoutSet     bool

	Chunker string
	Layout  Layout

	Pin      bool
	OnlyHash bool
	FsCache  bool
	NoCopy   bool

	Events   chan<- interface{}
	Silent   bool
	Progress bool

	PreserveMode  bool
	PreserveMtime bool
	Mode          os.FileMode
	Mtime         time.Time
}

type UnixfsLsSettings struct {
	ResolveChildren   bool
	UseCumulativeSize bool
}

type (
	UnixfsAddOption func(*UnixfsAddSettings) error
	UnixfsLsOption  func(*UnixfsLsSettings) error
)

func UnixfsAddOptions(opts ...UnixfsAddOption) (*UnixfsAddSettings, cid.Prefix, error) {
	options := &UnixfsAddSettings{
		CidVersion: -1,
		MhType:     mh.SHA2_256,

		Inline:               false,
		InlineLimit:          32,
		RawLeaves:            false,
		RawLeavesSet:         false,
		MaxFileLinks:         helpers.DefaultLinksPerBlock,
		MaxFileLinksSet:      false,
		MaxDirectoryLinks:    0,
		MaxDirectoryLinksSet: false,
		MaxHAMTFanout:        io.DefaultShardWidth,
		MaxHAMTFanoutSet:     false,

		Chunker: "size-262144",
		Layout:  BalancedLayout,

		Pin:      false,
		OnlyHash: false,
		FsCache:  false,
		NoCopy:   false,

		Events:   nil,
		Silent:   false,
		Progress: false,

		PreserveMode:  false,
		PreserveMtime: false,
		Mode:          0,
		Mtime:         time.Time{},
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, cid.Prefix{}, err
		}
	}

	// nocopy -> rawblocks
	if options.NoCopy && !options.RawLeaves {
		// fixed?
		if options.RawLeavesSet {
			return nil, cid.Prefix{}, fmt.Errorf("nocopy option requires '--raw-leaves' to be enabled as well")
		}

		// No, satisfy mandatory constraint.
		options.RawLeaves = true
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

	if !options.Mtime.IsZero() && options.PreserveMtime {
		options.PreserveMtime = false
	}

	if options.Mode != 0 && options.PreserveMode {
		options.PreserveMode = false
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

func UnixfsLsOptions(opts ...UnixfsLsOption) (*UnixfsLsSettings, error) {
	options := &UnixfsLsSettings{
		ResolveChildren: true,
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

// CidVersion specifies which CID version to use. Defaults to 0 unless an option
// that depends on CIDv1 is passed.
func (unixfsOpts) CidVersion(version int) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.CidVersion = version
		return nil
	}
}

// Hash function to use. Implies CIDv1 if not set to sha2-256 (default).
//
// Table of functions is declared in https://github.com/multiformats/go-multihash/blob/master/multihash.go
func (unixfsOpts) Hash(mhtype uint64) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.MhType = mhtype
		return nil
	}
}

// RawLeaves specifies whether to use raw blocks for leaves (data nodes with no
// links) instead of wrapping them with unixfs structures.
func (unixfsOpts) RawLeaves(enable bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.RawLeaves = enable
		settings.RawLeavesSet = true
		return nil
	}
}

// MaxFileLinks specifies the maximum number of children for UnixFS file
// nodes.
func (unixfsOpts) MaxFileLinks(n int) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.MaxFileLinks = n
		settings.MaxFileLinksSet = true
		return nil
	}
}

// MaxDirectoryLinks specifies the maximum number of children for UnixFS basic
// directory nodes.
func (unixfsOpts) MaxDirectoryLinks(n int) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.MaxDirectoryLinks = n
		settings.MaxDirectoryLinksSet = true
		return nil
	}
}

// MaxHAMTFanout specifies the maximum width of the HAMT directory shards.
func (unixfsOpts) MaxHAMTFanout(n int) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.MaxHAMTFanout = n
		settings.MaxHAMTFanoutSet = true
		return nil
	}
}

// Inline tells the adder to inline small blocks into CIDs
func (unixfsOpts) Inline(enable bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Inline = enable
		return nil
	}
}

// InlineLimit sets the amount of bytes below which blocks will be encoded
// directly into CID instead of being stored and addressed by it's hash.
// Specifying this option won't enable block inlining. For that use `Inline`
// option. Default: 32 bytes
//
// Note that while there is no hard limit on the number of bytes, it should be
// kept at a reasonably low value, such as 64; implementations may choose to
// reject anything larger.
func (unixfsOpts) InlineLimit(limit int) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.InlineLimit = limit
		return nil
	}
}

// Chunker specifies settings for the chunking algorithm to use.
//
// Default: size-262144, formats:
// size-[bytes] - Simple chunker splitting data into blocks of n bytes
// rabin-[min]-[avg]-[max] - Rabin chunker
func (unixfsOpts) Chunker(chunker string) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Chunker = chunker
		return nil
	}
}

// Layout tells the adder how to balance data between leaves.
// options.BalancedLayout is the default, it's optimized for static seekable
// files.
// options.TrickleLayout is optimized for streaming data,
func (unixfsOpts) Layout(layout Layout) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Layout = layout
		return nil
	}
}

// Pin tells the adder to pin the file root recursively after adding
func (unixfsOpts) Pin(pin bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Pin = pin
		return nil
	}
}

// HashOnly will make the adder calculate data hash without storing it in the
// blockstore or announcing it to the network
func (unixfsOpts) HashOnly(hashOnly bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.OnlyHash = hashOnly
		return nil
	}
}

// Events specifies channel which will be used to report events about ongoing
// Add operation.
//
// Note that if this channel blocks it may slowdown the adder
func (unixfsOpts) Events(sink chan<- interface{}) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Events = sink
		return nil
	}
}

// Silent reduces event output
func (unixfsOpts) Silent(silent bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Silent = silent
		return nil
	}
}

// Progress tells the adder whether to enable progress events
func (unixfsOpts) Progress(enable bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Progress = enable
		return nil
	}
}

// FsCache tells the adder to check the filestore for pre-existing blocks
//
// Experimental
func (unixfsOpts) FsCache(enable bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.FsCache = enable
		return nil
	}
}

// NoCopy tells the adder to add the files using filestore. Implies RawLeaves.
//
// Experimental
func (unixfsOpts) Nocopy(enable bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.NoCopy = enable
		return nil
	}
}

func (unixfsOpts) ResolveChildren(resolve bool) UnixfsLsOption {
	return func(settings *UnixfsLsSettings) error {
		settings.ResolveChildren = resolve
		return nil
	}
}

func (unixfsOpts) UseCumulativeSize(use bool) UnixfsLsOption {
	return func(settings *UnixfsLsSettings) error {
		settings.UseCumulativeSize = use
		return nil
	}
}

// PreserveMode tells the adder to store the file permissions
func (unixfsOpts) PreserveMode(enable bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.PreserveMode = enable
		return nil
	}
}

// PreserveMtime tells the adder to store the file modification time
func (unixfsOpts) PreserveMtime(enable bool) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.PreserveMtime = enable
		return nil
	}
}

// Mode represents a unix file mode
func (unixfsOpts) Mode(mode os.FileMode) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		settings.Mode = mode
		return nil
	}
}

// Mtime represents a unix file mtime
func (unixfsOpts) Mtime(seconds int64, nsecs uint32) UnixfsAddOption {
	return func(settings *UnixfsAddSettings) error {
		if nsecs > 999999999 {
			return errors.New("mtime nanoseconds must be in range [1, 999999999]")
		}
		settings.Mtime = time.Unix(seconds, int64(nsecs))
		return nil
	}
}
