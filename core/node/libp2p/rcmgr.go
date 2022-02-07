package libp2p

import (
	"errors"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
)

func ResourceManager() func() (Libp2pOpts, error) {
	return func() (opts Libp2pOpts, err error) {
		var limiter *rcmgr.BasicLimiter

		limitsIn, err := os.Open("./limits.json")
		switch {
		case err == nil:
			defer limitsIn.Close()
			limiter, err = rcmgr.NewDefaultLimiterFromJSON(limitsIn)
			if err != nil {
				return opts, fmt.Errorf("error parsing limit file: %w", err)
			}
		case errors.Is(err, os.ErrNotExist):
			limiter = rcmgr.NewDefaultLimiter()
		default:
			return opts, err
		}

		libp2p.SetDefaultServiceLimits(limiter)

		// TODO: close the resource manager when the node is shut down
		rcmgr, err := rcmgr.NewResourceManager(limiter)
		if err != nil {
			return opts, fmt.Errorf("error creating resource manager: %w", err)
		}
		opts.Opts = append(opts.Opts, libp2p.ResourceManager(rcmgr))
		return opts, nil
	}
}
