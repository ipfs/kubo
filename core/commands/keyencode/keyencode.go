package keyencode

import (
	peer "github.com/libp2p/go-libp2p-core/peer"
	mbase "github.com/multiformats/go-multibase"
)

const IPNSKeyFormatOptionName = "ipns-base"

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
