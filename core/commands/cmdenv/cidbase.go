package cmdenv

import (
	oldcmds "github.com/ipfs/go-ipfs/commands"

	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmXTmUCBtDUrzDYVzASogLiNph7EBuYqEgPL7QoHNMzUnz/go-ipfs-cmds"
	cidenc "gx/ipfs/QmdPF1WZQHFNfLdwhaShiR3e4KvFviAM58TrxVxPMhukic/go-cidutil/cidenc"
	mbase "gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"
)

var OptionCidBase = cmdkit.StringOption("cid-base", "mbase", "Multi-base to use to encode version 1 CIDs in output.")
var OptionOutputCidV1 = cmdkit.BoolOption("output-cidv1", "Upgrade CID version 0 to version 1 in output.")

type CidBaseHandler struct {
	base           string
	upgrade        bool
	upgradeDefined bool
	args           []string
	enc            *cidenc.Encoder
}

func NewCidBaseHandler(req *cmds.Request) *CidBaseHandler {
	h := &CidBaseHandler{}
	h.base, _ = req.Options["cid-base"].(string)
	h.upgrade, h.upgradeDefined = req.Options["output-cidv1"].(bool)
	h.args = req.Arguments
	return h
}

func NewCidBaseHandlerLegacy(req oldcmds.Request) *CidBaseHandler {
	h := &CidBaseHandler{}
	h.base, _, _ = req.Option("cid-base").String()
	h.upgrade, h.upgradeDefined, _ = req.Option("output-cidv1").Bool()
	h.args = req.Arguments()
	return h
}

func (h *CidBaseHandler) UseGlobal() *CidBaseHandler {
	h.enc = &cidenc.Default
	return h
}

func (h *CidBaseHandler) Proc() (*CidBaseHandler, error) {
	var e cidenc.Encoder = cidenc.Default
	if h.base != "" {
		var err error
		e.Base, err = mbase.EncoderByName(h.base)
		if err != nil {
			return h, err
		}
	}

	e.Upgrade = h.upgrade
	if h.base != "" && !h.upgradeDefined {
		e.Upgrade = true
	}

	if h.enc == nil {
		h.enc = &cidenc.Encoder{}
	}
	*h.enc = e
	return h, nil
}

func (h *CidBaseHandler) Encoder() cidenc.Encoder {
	return *h.enc
}

func (h *CidBaseHandler) EncoderFromPath(p string) cidenc.Encoder {
	if h.base == "" {
		enc, _ := cidenc.FromPath(*h.enc, p)
		return enc
	} else {
		return *h.enc
	}
}

func (h *CidBaseHandler) EncoderWithOverride() cidenc.Interface {
	if h.base == "" {
		enc := cidenc.NewOverride(*h.enc)
		enc.Add(h.args...)
		return enc
	} else {
		return *h.enc
	}
}
