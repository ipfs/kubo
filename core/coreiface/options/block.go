package options

import (
	"fmt"

	cid "github.com/ipfs/go-cid"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
)

type BlockPutSettings struct {
	CidPrefix cid.Prefix
	Pin       bool
}

type BlockRmSettings struct {
	Force bool
}

type (
	BlockPutOption func(*BlockPutSettings) error
	BlockRmOption  func(*BlockRmSettings) error
)

func BlockPutOptions(opts ...BlockPutOption) (*BlockPutSettings, error) {
	var cidPrefix cid.Prefix

	// Baseline is CIDv1 raw sha2-255-32 (can be tweaked later via opts)
	cidPrefix.Version = 1
	cidPrefix.Codec = uint64(mc.Raw)
	cidPrefix.MhType = mh.SHA2_256
	cidPrefix.MhLength = -1 // -1 means len is to be calculated during mh.Sum()

	options := &BlockPutSettings{
		CidPrefix: cidPrefix,
		Pin:       false,
	}

	// Apply any overrides
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

// CidCodec is the modern option for Block.Put which specifies the multicodec to use
// in the CID returned by the Block.Put operation.
// It uses correct codes from go-multicodec and replaces the old Format now with CIDv1 as the default.
func (blockOpts) CidCodec(codecName string) BlockPutOption {
	return func(settings *BlockPutSettings) error {
		if codecName == "" {
			return nil
		}
		code, err := codeFromName(codecName)
		if err != nil {
			return err
		}
		settings.CidPrefix.Codec = uint64(code)
		return nil
	}
}

// Map string to code from go-multicodec
func codeFromName(codecName string) (mc.Code, error) {
	var cidCodec mc.Code
	err := cidCodec.Set(codecName)
	return cidCodec, err
}

// Format is a legacy option for Block.Put which specifies the multicodec to
// use to serialize the object.
// Provided for backward-compatibility only. Use CidCodec instead.
func (blockOpts) Format(format string) BlockPutOption {
	return func(settings *BlockPutSettings) error {
		if format == "" {
			return nil
		}
		// Opt-in CIDv0 support for backward-compatibility
		if format == "v0" {
			settings.CidPrefix.Version = 0
		}

		// Fixup a legacy (invalid) names for dag-pb (0x70)
		if format == "v0" || format == "protobuf" {
			format = "dag-pb"
		}

		// Fixup invalid name for dag-cbor (0x71)
		if format == "cbor" {
			format = "dag-cbor"
		}

		// Set code based on name passed as "format"
		code, err := codeFromName(format)
		if err != nil {
			return err
		}
		settings.CidPrefix.Codec = uint64(code)

		// If CIDv0, ensure all parameters are compatible
		// (in theory go-cid would validate this anyway, but we want to provide better errors)
		pref := settings.CidPrefix
		if pref.Version == 0 {
			if pref.Codec != uint64(mc.DagPb) {
				return fmt.Errorf("only dag-pb is allowed with CIDv0")
			}
			if pref.MhType != mh.SHA2_256 || (pref.MhLength != -1 && pref.MhLength != 32) {
				return fmt.Errorf("only sha2-255-32 is allowed with CIDv0")
			}
		}

		return nil
	}
}

// Hash is an option for Block.Put which specifies the multihash settings to use
// when hashing the object. Default is mh.SHA2_256 (0x12).
// If mhLen is set to -1, default length for the hash will be used
func (blockOpts) Hash(mhType uint64, mhLen int) BlockPutOption {
	return func(settings *BlockPutSettings) error {
		settings.CidPrefix.MhType = mhType
		settings.CidPrefix.MhLength = mhLen
		return nil
	}
}

// Pin is an option for Block.Put which specifies whether to (recursively) pin
// added blocks
func (blockOpts) Pin(pin bool) BlockPutOption {
	return func(settings *BlockPutSettings) error {
		settings.Pin = pin
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
