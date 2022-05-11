package libp2p

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"strings"

	config "github.com/ipfs/go-ipfs/config"
	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"

	"github.com/wI2L/jsondiff"
)

// This file defines implicit limit defaults used when Swarm.ResourceMgr.Enabled

// adjustedDefaultLimits allows for tweaking defaults based on external factors,
// such as values in Swarm.ConnMgr.HiWater config.
func adjustedDefaultLimits(cfg config.SwarmConfig) rcmgr.DefaultLimitConfig {
	// Run checks to avoid introducing regressions
	if os.Getenv("IPFS_CHECK_RCMGR_DEFAULTS") != "" {
		// FIXME: Broken. Being tracked in https://github.com/ipfs/go-ipfs/issues/8949.
		checkImplicitDefaults()
	}

	// Return to use unmodified static limits based on values from go-libp2p 0.18
	// return defaultLimits

	// Adjust limits
	// (based on https://github.com/filecoin-project/lotus/pull/8318/files)
	// - give it more memory, up to 4G, min of 1G
	// - if Swarm.ConnMgr.HighWater is too high, adjust Conn/FD/Stream limits
	defaultLimits := rcmgr.DefaultLimits.WithSystemMemory(.125, 1<<30, 4<<30)

	// Do we need to adjust due to Swarm.ConnMgr.HighWater?
	if cfg.ConnMgr.Type == "basic" {
		maxconns := cfg.ConnMgr.HighWater
		if 2*maxconns > defaultLimits.SystemBaseLimit.ConnsInbound {
			// adjust conns to 2x to allow for two conns per peer (TCP+QUIC)
			defaultLimits.SystemBaseLimit.ConnsInbound = logScale(2 * maxconns)
			defaultLimits.SystemBaseLimit.ConnsOutbound = logScale(2 * maxconns)
			defaultLimits.SystemBaseLimit.Conns = logScale(4 * maxconns)

			defaultLimits.SystemBaseLimit.StreamsInbound = logScale(16 * maxconns)
			defaultLimits.SystemBaseLimit.StreamsOutbound = logScale(64 * maxconns)
			defaultLimits.SystemBaseLimit.Streams = logScale(64 * maxconns)

			if 2*maxconns > defaultLimits.SystemBaseLimit.FD {
				defaultLimits.SystemBaseLimit.FD = logScale(2 * maxconns)
			}

			defaultLimits.ServiceBaseLimit.StreamsInbound = logScale(8 * maxconns)
			defaultLimits.ServiceBaseLimit.StreamsOutbound = logScale(32 * maxconns)
			defaultLimits.ServiceBaseLimit.Streams = logScale(32 * maxconns)

			defaultLimits.ProtocolBaseLimit.StreamsInbound = logScale(8 * maxconns)
			defaultLimits.ProtocolBaseLimit.StreamsOutbound = logScale(32 * maxconns)
			defaultLimits.ProtocolBaseLimit.Streams = logScale(32 * maxconns)
		}
	}

	return defaultLimits
}

func logScale(val int) int {
	bitlen := bits.Len(uint(val))
	return 1 << bitlen
}

// checkImplicitDefaults compares libp2p defaults agains expected ones
// and panics when they don't match. This ensures we are not surprised
// by silent default limit changes when we update go-libp2p dependencies.
func checkImplicitDefaults() {
	ok := true

	// Check 1: did go-libp2p-resource-manager's DefaultLimits change?
	defaults, err := json.Marshal(rcmgr.DefaultLimits)
	if err != nil {
		log.Fatal(err)
	}
	changes, err := jsonDiff([]byte(expectedDefaultLimits), defaults)
	if err != nil {
		log.Fatal(err)
	}
	if len(changes) > 0 {
		ok = false
		log.Errorf("===> OOF! go-libp2p-resource-manager changed DefaultLimits\n"+
			"=> changes ('test' represents the old value):\n%s\n"+
			"=> go-libp2p-resource-manager DefaultLimits update needs a review:\n"+
			"Please inspect if changes impact go-ipfs users, and update expectedDefaultLimits in rcmgr_defaults.go to remove this message",
			strings.Join(changes, "\n"),
		)
	}

	// Check 2: did go-libp2p's SetDefaultServiceLimits change?
	testLimiter := rcmgr.NewStaticLimiter(rcmgr.DefaultLimits)
	libp2p.SetDefaultServiceLimits(testLimiter)

	serviceDefaults, err := json.Marshal(testLimiter)
	if err != nil {
		log.Fatal(err)
	}
	changes, err = jsonDiff([]byte(expectedDefaultServiceLimits), serviceDefaults)
	if err != nil {
		log.Fatal(err)
	}
	if len(changes) > 0 {
		ok = false
		log.Errorf("===> OOF! go-libp2p changed DefaultServiceLimits\n"+
			"=> changes ('test' represents the old value):\n%s\n"+
			"=> go-libp2p SetDefaultServiceLimits update needs a review:\n"+
			"Please inspect if changes impact go-ipfs users, and update expectedDefaultServiceLimits in rcmgr_defaults.go to remove this message",
			strings.Join(changes, "\n"),
		)
	}
	if !ok {
		log.Fatal("daemon will refuse to run with the resource manager until this is resolved")
	}
}

// jsonDiff compares two strings and returns diff in JSON Patch format
func jsonDiff(old []byte, updated []byte) ([]string, error) {
	// generate 'invertible' patch which includes old values as "test" op
	patch, err := jsondiff.CompareJSONOpts(old, updated, jsondiff.Invertible())
	changes := make([]string, len(patch))
	if err != nil {
		return changes, err
	}
	for i, op := range patch {
		changes[i] = fmt.Sprintf("  %s", op)
	}
	return changes, nil
}

// https://github.com/libp2p/go-libp2p-resource-manager/blob/v0.1.5/limit_defaults.go#L49
const expectedDefaultLimits = `{
  "SystemBaseLimit": {
    "Streams": 16384,
    "StreamsInbound": 4096,
    "StreamsOutbound": 16384,
    "Conns": 1024,
    "ConnsInbound": 256,
    "ConnsOutbound": 1024,
    "FD": 512
  },
  "SystemMemory": {
    "MemoryFraction": 0.125,
    "MinMemory": 134217728,
    "MaxMemory": 1073741824
  },
  "TransientBaseLimit": {
    "Streams": 512,
    "StreamsInbound": 128,
    "StreamsOutbound": 512,
    "Conns": 128,
    "ConnsInbound": 32,
    "ConnsOutbound": 128,
    "FD": 128
  },
  "TransientMemory": {
    "MemoryFraction": 1,
    "MinMemory": 67108864,
    "MaxMemory": 67108864
  },
  "ServiceBaseLimit": {
    "Streams": 8192,
    "StreamsInbound": 2048,
    "StreamsOutbound": 8192,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0
  },
  "ServiceMemory": {
    "MemoryFraction": 0.03125,
    "MinMemory": 67108864,
    "MaxMemory": 268435456
  },
  "ServicePeerBaseLimit": {
    "Streams": 512,
    "StreamsInbound": 256,
    "StreamsOutbound": 512,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0
  },
  "ServicePeerMemory": {
    "MemoryFraction": 0.0078125,
    "MinMemory": 16777216,
    "MaxMemory": 67108864
  },
  "ProtocolBaseLimit": {
    "Streams": 4096,
    "StreamsInbound": 1024,
    "StreamsOutbound": 4096,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0
  },
  "ProtocolMemory": {
    "MemoryFraction": 0.015625,
    "MinMemory": 67108864,
    "MaxMemory": 134217728
  },
  "ProtocolPeerBaseLimit": {
    "Streams": 512,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0
  },
  "ProtocolPeerMemory": {
    "MemoryFraction": 0.0078125,
    "MinMemory": 16777216,
    "MaxMemory": 67108864
  },
  "PeerBaseLimit": {
    "Streams": 1024,
    "StreamsInbound": 512,
    "StreamsOutbound": 1024,
    "Conns": 16,
    "ConnsInbound": 8,
    "ConnsOutbound": 16,
    "FD": 8
  },
  "PeerMemory": {
    "MemoryFraction": 0.0078125,
    "MinMemory": 67108864,
    "MaxMemory": 134217728
  },
  "ConnBaseLimit": {
    "Streams": 0,
    "StreamsInbound": 0,
    "StreamsOutbound": 0,
    "Conns": 1,
    "ConnsInbound": 1,
    "ConnsOutbound": 1,
    "FD": 1
  },
  "ConnMemory": 1048576,
  "StreamBaseLimit": {
    "Streams": 1,
    "StreamsInbound": 1,
    "StreamsOutbound": 1,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0
  },
  "StreamMemory": 16777216
}`

// https://github.com/libp2p/go-libp2p/blob/v0.18.0/limits.go#L17
const expectedDefaultServiceLimits = `{
  "SystemLimits": {
    "Streams": 16384,
    "StreamsInbound": 4096,
    "StreamsOutbound": 16384,
    "Conns": 1024,
    "ConnsInbound": 256,
    "ConnsOutbound": 1024,
    "FD": 512,
    "Memory": 1073741824
  },
  "TransientLimits": {
    "Streams": 512,
    "StreamsInbound": 128,
    "StreamsOutbound": 512,
    "Conns": 128,
    "ConnsInbound": 32,
    "ConnsOutbound": 128,
    "FD": 128,
    "Memory": 67108864
  },
  "DefaultServiceLimits": {
    "Streams": 8192,
    "StreamsInbound": 2048,
    "StreamsOutbound": 8192,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 67108864
  },
  "DefaultServicePeerLimits": {
    "Streams": 512,
    "StreamsInbound": 256,
    "StreamsOutbound": 512,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 16777216
  },
  "ServiceLimits": {
    "libp2p.autonat": {
      "Streams": 128,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "libp2p.holepunch": {
      "Streams": 256,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "libp2p.identify": {
      "Streams": 256,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "libp2p.ping": {
      "Streams": 128,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "libp2p.relay/v1": {
      "Streams": 1024,
      "StreamsInbound": 1024,
      "StreamsOutbound": 1024,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "libp2p.relay/v2": {
      "Streams": 1024,
      "StreamsInbound": 1024,
      "StreamsOutbound": 1024,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    }
  },
  "ServicePeerLimits": {
    "libp2p.autonat": {
      "Streams": 2,
      "StreamsInbound": 2,
      "StreamsOutbound": 2,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 557056
    },
    "libp2p.holepunch": {
      "Streams": 2,
      "StreamsInbound": 2,
      "StreamsOutbound": 2,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 557056
    },
    "libp2p.identify": {
      "Streams": 32,
      "StreamsInbound": 16,
      "StreamsOutbound": 16,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8912896
    },
    "libp2p.ping": {
      "Streams": 4,
      "StreamsInbound": 2,
      "StreamsOutbound": 3,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1114112
    },
    "libp2p.relay/v1": {
      "Streams": 64,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 17825792
    },
    "libp2p.relay/v2": {
      "Streams": 64,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 17825792
    }
  },
  "DefaultProtocolLimits": {
    "Streams": 4096,
    "StreamsInbound": 1024,
    "StreamsOutbound": 4096,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 67108864
  },
  "DefaultProtocolPeerLimits": {
    "Streams": 512,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 16777216
  },
  "ProtocolLimits": {
    "/ipfs/id/1.0.0": {
      "Streams": 4096,
      "StreamsInbound": 1024,
      "StreamsOutbound": 4096,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 33554432
    },
    "/ipfs/id/push/1.0.0": {
      "Streams": 4096,
      "StreamsInbound": 1024,
      "StreamsOutbound": 4096,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 33554432
    },
    "/ipfs/ping/1.0.0": {
      "Streams": 4096,
      "StreamsInbound": 1024,
      "StreamsOutbound": 4096,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "/libp2p/autonat/1.0.0": {
      "Streams": 4096,
      "StreamsInbound": 1024,
      "StreamsOutbound": 4096,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "/libp2p/circuit/relay/0.1.0": {
      "Streams": 1280,
      "StreamsInbound": 1280,
      "StreamsOutbound": 1280,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "/libp2p/circuit/relay/0.2.0/hop": {
      "Streams": 1280,
      "StreamsInbound": 1280,
      "StreamsOutbound": 1280,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "/libp2p/circuit/relay/0.2.0/stop": {
      "Streams": 1280,
      "StreamsInbound": 1280,
      "StreamsOutbound": 1280,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "/libp2p/dcutr": {
      "Streams": 4096,
      "StreamsInbound": 1024,
      "StreamsOutbound": 4096,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 67108864
    },
    "/p2p/id/delta/1.0.0": {
      "Streams": 4096,
      "StreamsInbound": 1024,
      "StreamsOutbound": 4096,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 33554432
    }
  },
  "ProtocolPeerLimits": {
    "/ipfs/id/1.0.0": {
      "Streams": 32,
      "StreamsInbound": 16,
      "StreamsOutbound": 16,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8912896
    },
    "/ipfs/id/push/1.0.0": {
      "Streams": 32,
      "StreamsInbound": 16,
      "StreamsOutbound": 16,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8912896
    },
    "/ipfs/ping/1.0.0": {
      "Streams": 4,
      "StreamsInbound": 2,
      "StreamsOutbound": 3,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1114112
    },
    "/libp2p/autonat/1.0.0": {
      "Streams": 2,
      "StreamsInbound": 2,
      "StreamsOutbound": 2,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 557056
    },
    "/libp2p/circuit/relay/0.1.0": {
      "Streams": 128,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 35651584
    },
    "/libp2p/circuit/relay/0.2.0/hop": {
      "Streams": 128,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 35651584
    },
    "/libp2p/circuit/relay/0.2.0/stop": {
      "Streams": 128,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 35651584
    },
    "/libp2p/dcutr": {
      "Streams": 2,
      "StreamsInbound": 2,
      "StreamsOutbound": 2,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 557056
    },
    "/p2p/id/delta/1.0.0": {
      "Streams": 32,
      "StreamsInbound": 16,
      "StreamsOutbound": 16,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8912896
    }
  },
  "DefaultPeerLimits": {
    "Streams": 1024,
    "StreamsInbound": 512,
    "StreamsOutbound": 1024,
    "Conns": 16,
    "ConnsInbound": 8,
    "ConnsOutbound": 16,
    "FD": 8,
    "Memory": 67108864
  },
  "PeerLimits": null,
  "ConnLimits": {
    "Streams": 0,
    "StreamsInbound": 0,
    "StreamsOutbound": 0,
    "Conns": 1,
    "ConnsInbound": 1,
    "ConnsOutbound": 1,
    "FD": 1,
    "Memory": 1048576
  },
  "StreamLimits": {
    "Streams": 1,
    "StreamsInbound": 1,
    "StreamsOutbound": 1,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 16777216
  }
}`
