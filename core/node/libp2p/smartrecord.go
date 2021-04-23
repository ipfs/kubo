package libp2p

import (
	libp2p "github.com/libp2p/go-libp2p-core"
	smart "github.com/libp2p/go-smart-record/protocol"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
)

func SmartRecordClient(lc fx.Lifecycle, mctx helpers.MetricsCtx, host libp2p.Host) (smart.SmartRecordClient, error) {
	ctx := helpers.LifecycleCtx(mctx, lc)
	return smart.NewSmartRecordClient(ctx, host)
}
func SmartRecordServer(lc fx.Lifecycle, mctx helpers.MetricsCtx, host libp2p.Host) (smart.SmartRecordServer, error) {
	ctx := helpers.LifecycleCtx(mctx, lc)
	return smart.NewSmartRecordServer(ctx, host)
}
