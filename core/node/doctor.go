package node

import (
	"context"
	"os"

	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/doctor"
	libp2p "github.com/libp2p/go-libp2p-core"
)

func Doctor(lc fx.Lifecycle, host libp2p.Host, cfg *config.Config) *doctor.Doctor {
	doctor := doctor.NewDoctor(host, cfg, os.Stdout)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return doctor.Start()
		},
		OnStop: func(ctx context.Context) error {
			return doctor.Close()
		},
	})
	return doctor
}
