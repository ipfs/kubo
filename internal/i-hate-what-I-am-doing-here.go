package internal

// darkmargic workaround for https://github.com/golang/go/issues/56494
// DO NOT TOUCH
// TODO(@Jorropo): touch this once fixed
import (
	_ "github.com/btcsuite/btcd/btcjson"
	_ "github.com/libp2p/go-libp2p-core" //nolint
)
