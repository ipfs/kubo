package libp2p

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ipfs/go-ipfs/repo"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
	"go.uber.org/fx"
)

const NetLimitDefaultFilename = "limit.json"

func ResourceManager() func(fx.Lifecycle, repo.Repo) (network.ResourceManager, Libp2pOpts, error) {
	return func(lc fx.Lifecycle, repo repo.Repo) (network.ResourceManager, Libp2pOpts, error) {
		var limiter *rcmgr.BasicLimiter
		var opts Libp2pOpts

		limitFile, err := os.Open(NetLimitDefaultFilename)
		if errors.Is(err, os.ErrNotExist) {
			log.Debug("limit file %s not found, creating a default resource manager", NetLimitDefaultFilename)
			limiter = rcmgr.NewDefaultLimiter()
		} else {
			if err != nil {
				return nil, opts, fmt.Errorf("error opening limit JSON file %s: %w",
					NetLimitDefaultFilename, err)
			}

			defer limitFile.Close() //nolint:errcheck
			limiter, err = rcmgr.NewDefaultLimiterFromJSON(limitFile)
			if err != nil {
				return nil, opts, fmt.Errorf("error parsing limit file: %w", err)
			}
		}

		libp2p.SetDefaultServiceLimits(limiter)

		var ropts []rcmgr.Option
		if os.Getenv("IPFS_DEBUG_RCMGR") != "" {
			ropts = append(ropts, rcmgr.WithTrace("rcmgr.json.gz"))
		}

		rcmgr, err := rcmgr.NewResourceManager(limiter, ropts...)
		if err != nil {
			return nil, opts, fmt.Errorf("error creating resource manager: %w", err)
		}
		opts.Opts = append(opts.Opts, libp2p.ResourceManager(rcmgr))

		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				return rcmgr.Close()
			}})

		return rcmgr, opts, nil
	}
}
