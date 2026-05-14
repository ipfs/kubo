package cmdenv

import (
	"fmt"
	"strings"

	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	mbase "github.com/multiformats/go-multibase"
)

var (
	OptionCidBase = cmds.StringOption("cid-base", "Multibase encoding for CIDs in output. CIDv0 is automatically converted to CIDv1 when a base other than base58btc is specified.")

	// OptionUpgradeCidV0InOutput is deprecated. When --cid-base is set to
	// anything other than base58btc, CIDv0 are now automatically upgraded
	// to CIDv1. This flag is kept for backward compatibility and will be
	// removed in a future release.
	OptionUpgradeCidV0InOutput = cmds.BoolOption("upgrade-cidv0-in-output", "[DEPRECATED] Upgrade version 0 to version 1 CIDs in output.")
)

// GetCidEncoder processes the --cid-base option and returns an encoder.
// When --cid-base is set to a non-base58btc encoding, CIDv0 values are
// automatically upgraded to CIDv1 because CIDv0 can only be represented
// in base58btc.
func GetCidEncoder(req *cmds.Request) (cidenc.Encoder, error) {
	base, _ := req.Options[OptionCidBase.Name()].(string)
	upgrade, upgradeDefined := req.Options[OptionUpgradeCidV0InOutput.Name()].(bool)

	e := cidenc.Default()

	if base != "" {
		var err error
		e.Base, err = mbase.EncoderByName(base)
		if err != nil {
			return e, err
		}
		// CIDv0 can only be represented in base58btc. When any other
		// base is requested, always upgrade CIDv0 to CIDv1 so the
		// output actually uses the requested encoding.
		if e.Base.Encoding() != mbase.Base58BTC {
			e.Upgrade = true
		}
	}

	// Deprecated: --upgrade-cidv0-in-output still works as an explicit
	// override for backward compatibility.
	if upgradeDefined {
		e.Upgrade = upgrade
	}

	return e, nil
}

// CidBaseDefined returns true if the `cid-base` option is specified on the
// command line
func CidBaseDefined(req *cmds.Request) bool {
	base, _ := req.Options["cid-base"].(string)
	return base != ""
}

// CidEncoderFromPath creates a new encoder that is influenced from the encoded
// Cid in a Path. For CIDv0 the multibase from the base encoder is used and
// automatic upgrades are disabled. For CIDv1 the multibase from the CID is
// used and upgrades are enabled.
//
// This logic is intentionally fuzzy and matches anything of the form
// `CidLike`, `CidLike/...`, or `/namespace/CidLike/...`.
//
// For example:
//
// * Qm...
// * Qm.../...
// * /ipfs/Qm...
// * /ipns/bafybeiahnxfi7fpmr5wtxs2imx4abnyn7fdxeiox7xxjem6zuiioqkh6zi/...
// * /bzz/bafybeiahnxfi7fpmr5wtxs2imx4abnyn7fdxeiox7xxjem6zuiioqkh6zi/...
func CidEncoderFromPath(p string) (cidenc.Encoder, error) {
	components := strings.SplitN(p, "/", 4)

	var maybeCid string
	if components[0] != "" {
		// No leading slash, first component is likely CID-like.
		maybeCid = components[0]
	} else if len(components) < 3 {
		// Not enough components to include a CID.
		return cidenc.Encoder{}, fmt.Errorf("no cid in path: %s", p)
	} else {
		maybeCid = components[2]
	}
	c, err := cid.Decode(maybeCid)
	if err != nil {
		// Ok, not a CID-like thing. Keep the current encoder.
		return cidenc.Encoder{}, fmt.Errorf("no cid in path: %s", p)
	}
	if c.Version() == 0 {
		// Version 0, use the base58 non-upgrading encoder.
		return cidenc.Default(), nil
	}

	// Version 1+, extract multibase encoding.
	encoding, _, err := mbase.Decode(maybeCid)
	if err != nil {
		// This should be impossible, we've already decoded the cid.
		panic(fmt.Sprintf("BUG: failed to get multibase decoder for CID %s", maybeCid))
	}

	return cidenc.Encoder{Base: mbase.MustNewEncoder(encoding), Upgrade: true}, nil
}
