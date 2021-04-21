package libp2p

import (
	libp2p "github.com/libp2p/go-libp2p-core"
	smart "github.com/libp2p/go-smart-record/protocol"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
)

func SmartRecord(serverMode bool) func(lc fx.Lifecycle, mctx helpers.MetricsCtx, host libp2p.Host) (smart.SmartRecordManager, error) {
	return func(lc fx.Lifecycle, mctx helpers.MetricsCtx, host libp2p.Host) (smart.SmartRecordManager, error) {
		ctx := helpers.LifecycleCtx(mctx, lc)
		if serverMode {
			return smart.NewSmartRecordManager(ctx, host)
		} else {
			return smart.NewSmartRecordClient(ctx, host)
		}
	}
}
