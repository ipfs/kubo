package cmdenv

import (
	oldcmds "github.com/ipfs/go-ipfs/commands"

	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmXTmUCBtDUrzDYVzASogLiNph7EBuYqEgPL7QoHNMzUnz/go-ipfs-cmds"
	cidenc "gx/ipfs/QmdPF1WZQHFNfLdwhaShiR3e4KvFviAM58TrxVxPMhukic/go-cidutil/cidenc"
	mbase "gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"
)

var OptionCidBase = cmdkit.StringOption("cid-base", "mbase", "Multi-base encoding used for version 1 CIDs in output.")
var OptionOutputCidV1 = cmdkit.BoolOption("output-cidv1", "Upgrade CID version 0 to version 1 in output.")

// CidBaseHandler is a helper class to process the `--cid-base` and
// `--output-cidv1` options.  In the future it may also be used to
// process relevant config settings.
//
// Several of its methods return the class itself in order to allow
// easy chaining, a typical usage would be
// `cmdenv.NewCidBaseHandler(req).UseGlobal().Proc()` or
// `cmdenv.NewCidBaseHandlerLegacy(req).Proc()`.
type CidBaseHandler struct {
	base           string
	upgrade        bool
	upgradeDefined bool
	args           []string
	enc            *cidenc.Encoder
}

// NewCidBaseHandler created a CidBaseHandler from a request
func NewCidBaseHandler(req *cmds.Request) *CidBaseHandler {
	h := &CidBaseHandler{}
	h.base, _ = req.Options["cid-base"].(string)
	h.upgrade, h.upgradeDefined = req.Options["output-cidv1"].(bool)
	h.args = req.Arguments
	return h
}

// NewCidBaseHandlerLegacy created a CidBaseHandler from a request
// using the old commands library
func NewCidBaseHandlerLegacy(req oldcmds.Request) *CidBaseHandler {
	h := &CidBaseHandler{}
	h.base, _, _ = req.Option("cid-base").String()
	h.upgrade, h.upgradeDefined, _ = req.Option("output-cidv1").Bool()
	h.args = req.Arguments()
	return h
}

// UseGlobal enables the use of the global default.  This is somewhat
// of a hack and should be used with care.  In particular it should
// only be used on the client side and not the server side.
func (h *CidBaseHandler) UseGlobal() *CidBaseHandler {
	h.enc = &cidenc.Default
	return h
}

// Proc processes the `--cid-base` and `--output-cidv1` options.  If
// UseGlobal was enabled, it will change the value of the global
// default.
func (h *CidBaseHandler) Proc() (*CidBaseHandler, error) {
	e := cidenc.Default

	if h.base != "" {
		var err error
		e.Base, err = mbase.EncoderByName(h.base)
		if err != nil {
			return h, err
		}
		if !h.upgradeDefined {
			e.Upgrade = true
		}
	}

	if h.upgradeDefined {
		e.Upgrade = h.upgrade
	}

	if h.enc == nil {
		h.enc = &cidenc.Encoder{}
	}
	*h.enc = e
	return h, nil
}

// Encoder returns a copy of the underlying Encoder
func (h *CidBaseHandler) Encoder() cidenc.Encoder {
	return *h.enc
}

// EncoderFromPath returns a new Encoder that will format CIDs like
// the one in the path if the `--cid-base` option is not used.  (If
// the `--cid-base` is used then a copy of the base encoder will be
// returned.)  In particular: if the path contains a version 1 CID
// then all CIDs will be outputting using the same multibase.  if the
// path contains a version 0 CID then version 0 CIDs will be outputted
// as is and version 1 cids will use the multibase from the base
// encoder
func (h *CidBaseHandler) EncoderFromPath(p string) cidenc.Encoder {
	if h.base == "" {
		enc, _ := cidenc.FromPath(*h.enc, p)
		return enc
	} else {
		return *h.enc
	}
}

// EncoderWithOverride returns a new encoder that will use the setting
// from the base encoder unless it is a CID that was specified on the
// command line and the `--cid-base` option was not used.  (If the
// `--cid-base` is used then a copy of the base encoder will be
// returned.) In that case the same CID string as specified on the
// command line will be used.
func (h *CidBaseHandler) EncoderWithOverride() cidenc.Interface {
	if h.base == "" {
		enc := cidenc.NewOverride(*h.enc)
		enc.Add(h.args...)
		return enc
	} else {
		return *h.enc
	}
}
