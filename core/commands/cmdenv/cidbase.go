package cmdenv

import (
	cidenc "gx/ipfs/QmVjZoEZg2oxXGFGjbD28x3gGN6ALHAW6BN2LKRUcaJ21i/go-cidutil/cidenc"
	cmds "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	mbase "gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"
)

var OptionCidBase = cmdkit.StringOption("cid-base", "Multi-base encoding used for version 1 CIDs in output.")
var OptionOutputCidV1 = cmdkit.BoolOption("output-cidv1", "Upgrade CID version 0 to version 1 in output.")

// ProcCidBase processes the `cid-base` and `output-cidv1` options and
// returns a encoder to use based on those parameters.
func ProcCidBase(req *cmds.Request) (cidenc.Encoder, error) {
	base, _ := req.Options["cid-base"].(string)
	upgrade, upgradeDefined := req.Options["output-cidv1"].(bool)

	var e cidenc.Encoder = cidenc.Default

	if base != "" {
		var err error
		e.Base, err = mbase.EncoderByName(base)
		if err != nil {
			return e, err
		}
		e.Upgrade = true
	}

	if upgradeDefined {
		e.Upgrade = upgrade
	}

	return e, nil
}

func CidBaseDefined(req *cmds.Request) bool {
	base, _ := req.Options["cid-base"].(string)
	return base != ""
}
