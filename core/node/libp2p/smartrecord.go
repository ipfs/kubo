package libp2p

import (
	libp2p "github.com/libp2p/go-libp2p-core"
	smart "github.com/libp2p/go-smart-record/protocol"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
)

func SmartRecord(lc fx.Lifecycle, mctx helpers.MetricsCtx, host libp2p.Host) (smart.SmartRecordManager, error) {
	ctx := helpers.LifecycleCtx(mctx, lc)
	return smart.NewSmartRecordManager(ctx, host)
}

// NOTE: If we choose to pass the smart record options using IPFS config, we may uncomment this function
// and comment the above, modifying `core/node/groups.go` SmartRecords maybeProvide line with:
// maybeProvide(SmartRecords(options...), cfg...)
// func SmartRecord(smartOptions ...smart.Option) interface{} {
//         return func(lc fx.Lifecycle, mctx helpers.MetricsCtx, host libp2p.Host) (smart.SmartRecordManager, error) {
//                 ctx := helpers.LifecycleCtx(mctx, lc)
//                 return smart.NewSmartRecordManager(ctx, host, smartOptions...)
//         }
// }
