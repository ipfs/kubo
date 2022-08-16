package libp2p

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"strings"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"

	"github.com/wI2L/jsondiff"
)

// This file defines implicit limit defaults used when Swarm.ResourceMgr.Enabled

// adjustedDefaultLimits allows for tweaking defaults based on external factors,
// such as values in Swarm.ConnMgr.HiWater config.
func adjustedDefaultLimits(cfg config.SwarmConfig) rcmgr.LimitConfig {
	// Run checks to avoid introducing regressions
	if os.Getenv("IPFS_CHECK_RCMGR_DEFAULTS") != "" {
		// FIXME: Broken. Being tracked in https://github.com/ipfs/go-ipfs/issues/8949.
		checkImplicitDefaults()
	}
	defaultLimits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&defaultLimits)

	// Adjust limits
	// (based on https://github.com/filecoin-project/lotus/pull/8318/files)
	// - if Swarm.ConnMgr.HighWater is too high, adjust Conn/FD/Stream limits

	// Outbound conns and FDs are set very high to allow for the accelerated DHT client to (re)load its routing table.
	// Currently it doesn't gracefully handle RM throttling--once it does we can lower these.
	// High outbound conn limits are considered less of a DoS risk than high inbound conn limits.
	// Also note that, due to the behavior of the accelerated DHT client, we don't need many streams, just conns.
	if minOutbound := 65536; defaultLimits.SystemBaseLimit.ConnsOutbound < minOutbound {
		defaultLimits.SystemBaseLimit.ConnsOutbound = minOutbound
	}
	if minFD := 4096; defaultLimits.SystemBaseLimit.FD < minFD {
		defaultLimits.SystemBaseLimit.FD = minFD
	}
	defaultLimitConfig := defaultLimits.AutoScale()

	// Do we need to adjust due to Swarm.ConnMgr.HighWater?
	if cfg.ConnMgr.Type == "basic" {
		maxconns := cfg.ConnMgr.HighWater
		if 2*maxconns > defaultLimitConfig.System.ConnsInbound {
			// adjust conns to 2x to allow for two conns per peer (TCP+QUIC)
			defaultLimitConfig.System.ConnsInbound = logScale(2 * maxconns)
			defaultLimitConfig.System.ConnsOutbound = logScale(2 * maxconns)
			defaultLimitConfig.System.Conns = logScale(4 * maxconns)

			defaultLimitConfig.System.StreamsInbound = logScale(16 * maxconns)
			defaultLimitConfig.System.StreamsOutbound = logScale(64 * maxconns)
			defaultLimitConfig.System.Streams = logScale(64 * maxconns)

			if 2*maxconns > defaultLimitConfig.System.FD {
				defaultLimitConfig.System.FD = logScale(2 * maxconns)
			}

			defaultLimitConfig.ServiceDefault.StreamsInbound = logScale(8 * maxconns)
			defaultLimitConfig.ServiceDefault.StreamsOutbound = logScale(32 * maxconns)
			defaultLimitConfig.ServiceDefault.Streams = logScale(32 * maxconns)

			defaultLimitConfig.ProtocolDefault.StreamsInbound = logScale(8 * maxconns)
			defaultLimitConfig.ProtocolDefault.StreamsOutbound = logScale(32 * maxconns)
			defaultLimitConfig.ProtocolDefault.Streams = logScale(32 * maxconns)

			log.Info("adjusted default resource manager limits")
		}

	}

	return defaultLimitConfig
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
	// We compare the baseline (min specs), and check if we went down in any limits.
	l := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&l)
	limits := l.AutoScale()
	testLimiter := rcmgr.NewFixedLimiter(limits)

	serviceDefaults, err := json.Marshal(testLimiter)
	if err != nil {
		log.Fatal(err)
	}
	changes, err = jsonDiff([]byte(expectedDefaultServiceLimits), serviceDefaults)
	if err != nil {
		log.Fatal(err)
	}
	if len(changes) > 0 {
		oldState := map[string]int{}
		type Op struct {
			Op    string
			Path  string
			Value int
		}
		for _, changeStr := range changes {
			change := Op{}
			err := json.Unmarshal([]byte(changeStr), &change)
			if err != nil {
				continue
			}
			if change.Op == "test" {
				oldState[change.Path] = change.Value
			}
		}

		for _, changeStr := range changes {
			change := Op{}
			err := json.Unmarshal([]byte(changeStr), &change)
			if err != nil {
				continue
			}
			if change.Op == "replace" {
				oldVal, okFound := oldState[change.Path]
				if okFound && oldVal > change.Value {
					ok = false
					fmt.Printf("reduced value for %s. Old: %v; new: %v\n", change.Path, oldVal, change.Value)
				}
			}
		}

		if !ok {
			log.Errorf("===> OOF! go-libp2p reduced DefaultServiceLimits\n" +
				"=> See the aboce reduced values for info.\n" +
				"=> go-libp2p SetDefaultServiceLimits update needs a review:\n" +
				"Please inspect if changes impact go-ipfs users, and update expectedDefaultServiceLimits in rcmgr_defaults.go to remove this message",
			)
		}
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
    "Streams": 2048,
    "StreamsInbound": 1024,
    "StreamsOutbound": 2048,
    "Conns": 128,
    "ConnsInbound": 64,
    "ConnsOutbound": 128,
    "FD": 256,
    "Memory": 134217728
  },
  "SystemLimitIncrease": {
    "Streams": 2048,
    "StreamsInbound": 1024,
    "StreamsOutbound": 2048,
    "Conns": 128,
    "ConnsInbound": 64,
    "ConnsOutbound": 128,
    "Memory": 1073741824,
    "FDFraction": 1
  },
  "TransientBaseLimit": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 64,
    "ConnsInbound": 32,
    "ConnsOutbound": 64,
    "FD": 64,
    "Memory": 33554432
  },
  "TransientLimitIncrease": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 32,
    "ConnsInbound": 16,
    "ConnsOutbound": 32,
    "Memory": 134217728,
    "FDFraction": 0.25
  },
  "AllowlistedSystemBaseLimit": {
    "Streams": 2048,
    "StreamsInbound": 1024,
    "StreamsOutbound": 2048,
    "Conns": 128,
    "ConnsInbound": 64,
    "ConnsOutbound": 128,
    "FD": 256,
    "Memory": 134217728
  },
  "AllowlistedSystemLimitIncrease": {
    "Streams": 2048,
    "StreamsInbound": 1024,
    "StreamsOutbound": 2048,
    "Conns": 128,
    "ConnsInbound": 64,
    "ConnsOutbound": 128,
    "Memory": 1073741824,
    "FDFraction": 1
  },
  "AllowlistedTransientBaseLimit": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 64,
    "ConnsInbound": 32,
    "ConnsOutbound": 64,
    "FD": 64,
    "Memory": 33554432
  },
  "AllowlistedTransientLimitIncrease": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 32,
    "ConnsInbound": 16,
    "ConnsOutbound": 32,
    "Memory": 134217728,
    "FDFraction": 0.25
  },
  "ServiceBaseLimit": {
    "Streams": 4096,
    "StreamsInbound": 1024,
    "StreamsOutbound": 4096,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 67108864
  },
  "ServiceLimitIncrease": {
    "Streams": 2048,
    "StreamsInbound": 512,
    "StreamsOutbound": 2048,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "Memory": 134217728,
    "FDFraction": 0
  },
  "ServiceLimits": null,
  "ServicePeerBaseLimit": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 16777216
  },
  "ServicePeerLimitIncrease": {
    "Streams": 8,
    "StreamsInbound": 4,
    "StreamsOutbound": 8,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "Memory": 4194304,
    "FDFraction": 0
  },
  "ServicePeerLimits": null,
  "ProtocolBaseLimit": {
    "Streams": 2048,
    "StreamsInbound": 512,
    "StreamsOutbound": 2048,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 67108864
  },
  "ProtocolLimitIncrease": {
    "Streams": 512,
    "StreamsInbound": 256,
    "StreamsOutbound": 512,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "Memory": 171966464,
    "FDFraction": 0
  },
  "ProtocolLimits": null,
  "ProtocolPeerBaseLimit": {
    "Streams": 256,
    "StreamsInbound": 64,
    "StreamsOutbound": 128,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 16777216
  },
  "ProtocolPeerLimitIncrease": {
    "Streams": 16,
    "StreamsInbound": 4,
    "StreamsOutbound": 8,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "Memory": 4,
    "FDFraction": 0
  },
  "ProtocolPeerLimits": null,
  "PeerBaseLimit": {
    "Streams": 512,
    "StreamsInbound": 256,
    "StreamsOutbound": 512,
    "Conns": 8,
    "ConnsInbound": 4,
    "ConnsOutbound": 8,
    "FD": 4,
    "Memory": 67108864
  },
  "PeerLimitIncrease": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "Memory": 134217728,
    "FDFraction": 0.015625
  },
  "PeerLimits": null,
  "ConnBaseLimit": {
    "Streams": 0,
    "StreamsInbound": 0,
    "StreamsOutbound": 0,
    "Conns": 1,
    "ConnsInbound": 1,
    "ConnsOutbound": 1,
    "FD": 1,
    "Memory": 1048576
  },
  "ConnLimitIncrease": {
    "Streams": 0,
    "StreamsInbound": 0,
    "StreamsOutbound": 0,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "Memory": 0,
    "FDFraction": 0
  },
  "StreamBaseLimit": {
    "Streams": 1,
    "StreamsInbound": 1,
    "StreamsOutbound": 1,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 16777216
  },
  "StreamLimitIncrease": {
    "Streams": 0,
    "StreamsInbound": 0,
    "StreamsOutbound": 0,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "Memory": 0,
    "FDFraction": 0
  }
}`

// Generated from the default limits and scaling to 0 (base limit).
const expectedDefaultServiceLimits = `{
  "System": {
    "Streams": 2048,
    "StreamsInbound": 1024,
    "StreamsOutbound": 2048,
    "Conns": 128,
    "ConnsInbound": 64,
    "ConnsOutbound": 128,
    "FD": 256,
    "Memory": 134217728
  },
  "Transient": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 64,
    "ConnsInbound": 32,
    "ConnsOutbound": 64,
    "FD": 64,
    "Memory": 33554432
  },
  "AllowlistedSystem": {
    "Streams": 2048,
    "StreamsInbound": 1024,
    "StreamsOutbound": 2048,
    "Conns": 128,
    "ConnsInbound": 64,
    "ConnsOutbound": 128,
    "FD": 256,
    "Memory": 134217728
  },
  "AllowlistedTransient": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 64,
    "ConnsInbound": 32,
    "ConnsOutbound": 64,
    "FD": 64,
    "Memory": 33554432
  },
  "ServiceDefault": {
    "Streams": 4096,
    "StreamsInbound": 1024,
    "StreamsOutbound": 4096,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 67108864
  },
  "Service": {
    "libp2p.autonat": {
      "Streams": 64,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "libp2p.holepunch": {
      "Streams": 64,
      "StreamsInbound": 32,
      "StreamsOutbound": 32,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "libp2p.identify": {
      "Streams": 128,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "libp2p.ping": {
      "Streams": 64,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "libp2p.relay/v1": {
      "Streams": 256,
      "StreamsInbound": 256,
      "StreamsOutbound": 256,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 16777216
    },
    "libp2p.relay/v2": {
      "Streams": 256,
      "StreamsInbound": 256,
      "StreamsOutbound": 256,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 16777216
    }
  },
  "ServicePeerDefault": {
    "Streams": 256,
    "StreamsInbound": 128,
    "StreamsOutbound": 256,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 16777216
  },
  "ServicePeer": {
    "libp2p.autonat": {
      "Streams": 2,
      "StreamsInbound": 2,
      "StreamsOutbound": 2,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1048576
    },
    "libp2p.holepunch": {
      "Streams": 2,
      "StreamsInbound": 2,
      "StreamsOutbound": 2,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1048576
    },
    "libp2p.identify": {
      "Streams": 32,
      "StreamsInbound": 16,
      "StreamsOutbound": 16,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1048576
    },
    "libp2p.ping": {
      "Streams": 4,
      "StreamsInbound": 2,
      "StreamsOutbound": 3,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8590458880
    },
    "libp2p.relay/v1": {
      "Streams": 64,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1048576
    },
    "libp2p.relay/v2": {
      "Streams": 64,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1048576
    }
  },
  "ProtocolDefault": {
    "Streams": 2048,
    "StreamsInbound": 512,
    "StreamsOutbound": 2048,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 67108864
  },
  "Protocol": {
    "/ipfs/id/1.0.0": {
      "Streams": 128,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "/ipfs/id/push/1.0.0": {
      "Streams": 128,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "/ipfs/ping/1.0.0": {
      "Streams": 64,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "/libp2p/autonat/1.0.0": {
      "Streams": 64,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "/libp2p/circuit/relay/0.1.0": {
      "Streams": 640,
      "StreamsInbound": 640,
      "StreamsOutbound": 640,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 16777216
    },
    "/libp2p/circuit/relay/0.2.0/hop": {
      "Streams": 640,
      "StreamsInbound": 640,
      "StreamsOutbound": 640,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 16777216
    },
    "/libp2p/circuit/relay/0.2.0/stop": {
      "Streams": 640,
      "StreamsInbound": 640,
      "StreamsOutbound": 640,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 16777216
    },
    "/libp2p/dcutr": {
      "Streams": 64,
      "StreamsInbound": 32,
      "StreamsOutbound": 32,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    },
    "/p2p/id/delta/1.0.0": {
      "Streams": 128,
      "StreamsInbound": 64,
      "StreamsOutbound": 64,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 4194304
    }
  },
  "ProtocolPeerDefault": {
    "Streams": 256,
    "StreamsInbound": 64,
    "StreamsOutbound": 128,
    "Conns": 0,
    "ConnsInbound": 0,
    "ConnsOutbound": 0,
    "FD": 0,
    "Memory": 16777216
  },
  "ProtocolPeer": {
    "/ipfs/id/1.0.0": {
      "Streams": 32,
      "StreamsInbound": 16,
      "StreamsOutbound": 16,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8590458880
    },
    "/ipfs/id/push/1.0.0": {
      "Streams": 32,
      "StreamsInbound": 16,
      "StreamsOutbound": 16,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8590458880
    },
    "/ipfs/ping/1.0.0": {
      "Streams": 4,
      "StreamsInbound": 2,
      "StreamsOutbound": 3,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8590458880
    },
    "/libp2p/autonat/1.0.0": {
      "Streams": 2,
      "StreamsInbound": 2,
      "StreamsOutbound": 2,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1048576
    },
    "/libp2p/circuit/relay/0.1.0": {
      "Streams": 128,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 33554432
    },
    "/libp2p/circuit/relay/0.2.0/hop": {
      "Streams": 128,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 33554432
    },
    "/libp2p/circuit/relay/0.2.0/stop": {
      "Streams": 128,
      "StreamsInbound": 128,
      "StreamsOutbound": 128,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 33554432
    },
    "/libp2p/dcutr": {
      "Streams": 2,
      "StreamsInbound": 2,
      "StreamsOutbound": 2,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 1048576
    },
    "/p2p/id/delta/1.0.0": {
      "Streams": 32,
      "StreamsInbound": 16,
      "StreamsOutbound": 16,
      "Conns": 0,
      "ConnsInbound": 0,
      "ConnsOutbound": 0,
      "FD": 0,
      "Memory": 8590458880
    }
  },
  "PeerDefault": {
    "Streams": 512,
    "StreamsInbound": 256,
    "StreamsOutbound": 512,
    "Conns": 8,
    "ConnsInbound": 4,
    "ConnsOutbound": 8,
    "FD": 4,
    "Memory": 67108864
  },
  "Conn": {
    "Streams": 0,
    "StreamsInbound": 0,
    "StreamsOutbound": 0,
    "Conns": 1,
    "ConnsInbound": 1,
    "ConnsOutbound": 1,
    "FD": 1,
    "Memory": 1048576
  },
  "Stream": {
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
