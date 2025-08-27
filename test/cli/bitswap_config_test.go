package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestBitswapConfig(t *testing.T) {
	t.Parallel()

	// Create test data that will be shared between nodes
	testData := random.Bytes(100)

	t.Run("server enabled (default)", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		provider := h.NewNode().Init().StartDaemon()
		requester := h.NewNode().Init().StartDaemon()

		hash := provider.IPFSAddStr(string(testData))
		requester.Connect(provider)

		res := requester.IPFS("cat", hash)
		assert.Equal(t, testData, res.Stdout.Bytes(), "retrieved data should match original")
	})

	t.Run("server disabled", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		provider := h.NewNode().Init()
		provider.SetIPFSConfig("Bitswap.ServerEnabled", false)
		provider = provider.StartDaemon()

		requester := h.NewNode().Init().StartDaemon()

		hash := provider.IPFSAddStr(string(testData))
		requester.Connect(provider)

		// If the data was available, it would be retrieved immediately.
		// Therefore, after the timeout, we can assume the data is not available
		// i.e. the server is disabled
		timeout := time.After(3 * time.Second)
		dataChan := make(chan []byte)

		go func() {
			res := requester.RunIPFS("cat", hash)
			dataChan <- res.Stdout.Bytes()
		}()

		select {
		case data := <-dataChan:
			assert.NotEqual(t, testData, data, "retrieved data should not match original")
		case <-timeout:
			t.Log("Test passed: operation timed out after 3 seconds as expected")
		}
	})

	t.Run("client still works when server disabled", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		requester := h.NewNode().Init()
		requester.SetIPFSConfig("Bitswap.ServerEnabled", false)
		requester.StartDaemon()

		provider := h.NewNode().Init().StartDaemon()
		hash := provider.IPFSAddStr(string(testData))
		requester.Connect(provider)

		// Even when the server is disabled, the client should be able to retrieve data
		res := requester.RunIPFS("cat", hash)
		assert.Equal(t, testData, res.Stdout.Bytes(), "retrieved data should match original")
	})

	t.Run("bitswap over libp2p disabled", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		requester := h.NewNode().Init()
		requester.UpdateConfig(func(cfg *config.Config) {
			cfg.Bitswap.Libp2pEnabled = config.False
			cfg.Bitswap.ServerEnabled = config.False
			cfg.HTTPRetrieval.Enabled = config.True
		})
		requester.StartDaemon()

		provider := h.NewNode().Init().StartDaemon()
		hash := provider.IPFSAddStr(string(testData))

		requester.Connect(provider)
		res := requester.RunIPFS("cat", hash)
		assert.Equal(t, []byte{}, res.Stdout.Bytes(), "cat should not return any data")
		assert.Contains(t, res.Stderr.String(), "Error: ipld: could not find")

		// Verify that basic operations still work with bitswap disabled
		res = requester.IPFS("id")
		assert.Equal(t, 0, res.ExitCode(), "basic IPFS operations should work")
		res = requester.IPFS("bitswap", "stat")
		assert.Equal(t, 0, res.ExitCode(), "bitswap stat should work even with bitswap disabled")
		res = requester.IPFS("bitswap", "wantlist")
		assert.Equal(t, 0, res.ExitCode(), "bitswap wantlist should work even with bitswap disabled")

		// Verify local operations still work
		hashNew := requester.IPFSAddStr("random")
		res = requester.IPFS("cat", hashNew)
		assert.Equal(t, []byte("random"), res.Stdout.Bytes(), "cat should return the added data")
	})

	// Disabling Bitswap.Libp2pEnabled should remove /ipfs/bitswap* protocols from `ipfs id`
	t.Run("disabling bitswap over libp2p removes it from identify protocol list", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		provider := h.NewNode().Init()
		provider.UpdateConfig(func(cfg *config.Config) {
			cfg.Bitswap.Libp2pEnabled = config.False
			cfg.Bitswap.ServerEnabled = config.False
			cfg.HTTPRetrieval.Enabled = config.True
		})
		provider = provider.StartDaemon()
		requester := h.NewNode().Init().StartDaemon()
		requester.Connect(provider)

		// read libp2p identify from remote peer, and print protocols
		res := requester.IPFS("id", "-f", "<protocols>", provider.PeerID().String())
		protocols := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")

		// No bitswap protocols should be present
		for _, proto := range protocols {
			assert.NotContains(t, proto, bsnet.ProtocolBitswap, "bitswap protocol %s should not be advertised when server is disabled", proto)
			assert.NotContains(t, proto, bsnet.ProtocolBitswapNoVers, "bitswap protocol %s should not be advertised when server is disabled", proto)
			assert.NotContains(t, proto, bsnet.ProtocolBitswapOneOne, "bitswap protocol %s should not be advertised when server is disabled", proto)
			assert.NotContains(t, proto, bsnet.ProtocolBitswapOneZero, "bitswap protocol %s should not be advertised when server is disabled", proto)
		}
	})

	// HTTPRetrieval uses bitswap engine, we need it
	t.Run("errors when both HTTP and libp2p are disabled", func(t *testing.T) {
		t.Parallel()

		// init Kubo repo
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.HTTPRetrieval.Enabled = config.False
			cfg.Bitswap.Libp2pEnabled = config.False
			cfg.Bitswap.ServerEnabled = config.Default
		})
		res := node.RunIPFS("daemon")
		assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Bitswap.Libp2pEnabled and HTTPRetrieval.Enabled are both disabled, unable to initialize Bitswap")
		assert.Equal(t, 1, res.ExitCode())
	})

	// HTTPRetrieval uses bitswap engine, we need it
	t.Run("errors when user set conflicting HTTP and libp2p flags", func(t *testing.T) {
		t.Parallel()

		// init Kubo repo
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.HTTPRetrieval.Enabled = config.False
			cfg.Bitswap.Libp2pEnabled = config.False
			cfg.Bitswap.ServerEnabled = config.True // bad user config: can't enable server when libp2p is down
		})
		res := node.RunIPFS("daemon")
		assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Bitswap.Libp2pEnabled and HTTPRetrieval.Enabled are both disabled, unable to initialize Bitswap")
		assert.Equal(t, 1, res.ExitCode())
	})
}
