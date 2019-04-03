package core

import (
	"context"

	"github.com/jbenet/goprocess"
	"go.uber.org/fx"

	iconfig "github.com/ipfs/go-ipfs-config"
	uio "github.com/ipfs/go-unixfs/io"
)

////////////////////
// libp2p

func setupSharding(cfg *iconfig.Config) {
	// TEMP: setting global sharding switch here
	uio.UseHAMTSharding = cfg.Experimental.ShardingEnabled
}

func baseProcess(lc fx.Lifecycle) goprocess.Process {
	p := goprocess.WithParent(goprocess.Background())
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return p.Close()
		},
	})
	return p
}
