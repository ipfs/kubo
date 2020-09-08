package keyencode

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
	peer "github.com/libp2p/go-libp2p-core/peer"
	mbase "github.com/multiformats/go-multibase"
)

const ipnsKeyFormatOptionName = "ipns-base"

var OptionIPNSBase = cmds.StringOption(ipnsKeyFormatOptionName, "Encoding used for keys: Can either be a multibase encoded CID or a base58btc encoded multihash. Takes {b58mh|base36|k|base32|b...}.").WithDefault("base36")

type KeyEncoder struct {
	baseEnc *mbase.Encoder
}

func KeyEncoderFromString(formatLabel string) (KeyEncoder, error) {
	switch formatLabel {
	case "b58mh", "v0":
		return KeyEncoder{}, nil
	default:
		if enc, err := mbase.EncoderByName(formatLabel); err != nil {
			return KeyEncoder{}, err
		} else {
			return KeyEncoder{&enc}, nil
		}
	}
}

func (enc KeyEncoder) FormatID(id peer.ID) string {
	if enc.baseEnc == nil {
		//nolint deprecated
		return peer.IDB58Encode(id)
	}
	if s, err := peer.ToCid(id).StringOfBase(enc.baseEnc.Encoding()); err != nil {
		panic(err)
	} else {
		return s
	}
}
