package libp2p

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggingResourceManager(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		orig := rcmgr.DefaultLimits.AutoScale()
		limits := orig.ToPartialLimitConfig()
		limits.System.Conns = 1
		limits.System.ConnsInbound = 1
		limits.System.ConnsOutbound = 1
		limiter := rcmgr.NewFixedLimiter(limits.Build(orig))
		rm, err := rcmgr.NewResourceManager(limiter)
		if err != nil {
			t.Fatal(err)
		}
		defer rm.Close()

		oCore, oLogs := observer.New(zap.WarnLevel)
		oLogger := zap.New(oCore)
		lrm := &loggingResourceManager{
			logger:      oLogger.Sugar(),
			delegate:    rm,
			logInterval: 1 * time.Second,
		}

		// 2 of these should result in resource limit exceeded errors and subsequent log messages
		for i := 0; i < 3; i++ {
			_, _ = lrm.OpenConnection(network.DirInbound, false, ma.StringCast("/ip4/127.0.0.1/tcp/1234"))
		}

		// run the logger which will write an entry for those errors
		ctx := t.Context()
		lrm.start(ctx)
		time.Sleep(3 * time.Second)

		timer := time.NewTimer(1 * time.Second)
		for {
			select {
			case <-timer.C:
				t.Fatalf("expected logs never arrived")
			default:
				if oLogs.Len() == 0 {
					continue
				}
				require.Equal(t, "Protected from exceeding resource limits 2 times.  libp2p message: \"system: cannot reserve inbound connection: resource limit exceeded\".", oLogs.All()[0].Message)
				return
			}
		}
	})
}
