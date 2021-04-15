package libp2p

import (
	smart "github.com/libp2p/go-smart-record/protocol"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
	libp2p "github.com/libp2p/go-libp2p-core"
)

func SmartRecord(smartOptions ...smart.Option) interface{} {
	return func(lc fx.Lifecycle, mctx helpers.MetricsCtx, host libp2p.Host) (smart.SmartRecordManager, error) {
		ctx := helpers.LifecycleCtx(mctx, lc)
		return smart.NewSmartRecordManager(ctx, host, smartOptions...)
	}
}
