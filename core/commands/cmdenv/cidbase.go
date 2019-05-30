package cmdenv

import (
	"fmt"
	"strings"

	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	mbase "github.com/multiformats/go-multibase"
)

var OptionCidBase = cmds.StringOption("cid-base", "Multibase encoding used for version 1 CIDs in output.")
var OptionUpgradeCidV0InOutput = cmds.BoolOption("upgrade-cidv0-in-output", "Upgrade version 0 to version 1 CIDs in output.")

// GetCidEncoder processes the `cid-base` and `output-cidv1` options and
// returns a encoder to use based on those parameters.
func GetCidEncoder(req *cmds.Request) (cidenc.Encoder, error) {
	return getCidBase(req, true)
}

// GetLowLevelCidEncoder is like GetCidEncoder but meant to be used by
// lower level commands.  It differs from GetCidEncoder in that CIDv0
// are not, by default, auto-upgraded to CIDv1.
func GetLowLevelCidEncoder(req *cmds.Request) (cidenc.Encoder, error) {
	return getCidBase(req, false)
}

func getCidBase(req *cmds.Request, autoUpgrade bool) (cidenc.Encoder, error) {
	base, _ := req.Options[OptionCidBase.Name()].(string)
	upgrade, upgradeDefined := req.Options[OptionUpgradeCidV0InOutput.Name()].(bool)

	e := cidenc.Default()

	if base != "" {
		var err error
		e.Base, err = mbase.EncoderByName(base)
		if err != nil {
			return e, err
		}
		if autoUpgrade {
			e.Upgrade = true
		}
	}

	if upgradeDefined {
		e.Upgrade = upgrade
	}

	return e, nil
}

// CidBaseDefined returns true if the `cid-base` option is specified
// on the command line
func CidBaseDefined(req *cmds.Request) bool {
	base, _ := req.Options["cid-base"].(string)
	return base != ""
}

// CidEncoderFromPath creates a new encoder that is influenced from
// the encoded Cid in a Path.  For CidV0 the multibase from the base
// encoder is used and automatic upgrades are disabled.  For CidV1 the
// multibase from the CID is used and upgrades are enabled.
//
// This logic is intentionally fuzzy and will match anything of the form
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
