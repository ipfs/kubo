package runtime

import (
	"runtime"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/codahale/metrics"
)

func init() {
	metrics.Gauge("Goroutines.Num").SetFunc(func() int64 {
		return int64(runtime.NumGoroutine())
	})
}
