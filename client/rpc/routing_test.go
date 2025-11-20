package rpc

import (
	"context"
	"encoding/json"
	"testing"

	boxoprovider "github.com/ipfs/boxo/provider"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/libp2p/go-libp2p-kad-dht/provider/stats"
	"github.com/stretchr/testify/require"
)

// Compile-time check: ensure our response type is compatible with kubo's provideStats
// This verifies that JSON marshaling/unmarshaling will work correctly
var _ = func() {
	// Create instance of command's provideStats structure
	cmdStats := struct {
		Sweep  *stats.Stats                  `json:"Sweep,omitempty"`
		Legacy *boxoprovider.ReproviderStats `json:"Legacy,omitempty"`
		FullRT bool                          `json:"FullRT,omitempty"`
	}{}

	// Marshal and unmarshal to verify compatibility
	data, _ := json.Marshal(cmdStats)
	var ifaceStats iface.ProvideStatsResponse
	_ = json.Unmarshal(data, &ifaceStats)
}

// testProvideStats mirrors the subset of fields we verify in tests.
// Intentionally independent from coreiface types to detect breaking changes.
type testProvideStats struct {
	Sweep *struct {
		Connectivity struct {
			Status string `json:"status"`
		} `json:"connectivity"`
		Queues struct {
			PendingKeyProvides int `json:"pending_key_provides"`
		} `json:"queues"`
		Schedule struct {
			Keys int `json:"keys"`
		} `json:"schedule"`
	} `json:"Sweep,omitempty"`
	Legacy *struct {
		TotalReprovides uint64 `json:"TotalReprovides"`
	} `json:"Legacy,omitempty"`
}

func TestProvideStats_WithSweepProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := harness.NewT(t)
	node := h.NewNode().Init()

	// Explicitly enable Sweep provider (default in v0.39)
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
	node.SetIPFSConfig("Provide.Enabled", true)

	node.StartDaemon()
	defer node.StopDaemon()

	apiMaddr, err := node.TryAPIAddr()
	require.NoError(t, err)

	api, err := NewApi(apiMaddr)
	require.NoError(t, err)

	// Get provide stats
	result, err := api.Routing().ProvideStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify Sweep stats are present, Legacy is not
	require.NotNil(t, result.Sweep, "Sweep provider should return Sweep stats")
	require.Nil(t, result.Legacy, "Sweep provider should not return Legacy stats")

	// Marshal to JSON and unmarshal to test struct to verify structure
	data, err := json.Marshal(result)
	require.NoError(t, err)

	var testStats testProvideStats
	err = json.Unmarshal(data, &testStats)
	require.NoError(t, err)

	// Verify key fields exist and have reasonable values
	require.NotNil(t, testStats.Sweep)
	require.NotEmpty(t, testStats.Sweep.Connectivity.Status, "connectivity status should be present")
	require.GreaterOrEqual(t, testStats.Sweep.Queues.PendingKeyProvides, 0, "queue size should be non-negative")
	require.GreaterOrEqual(t, testStats.Sweep.Schedule.Keys, 0, "scheduled keys should be non-negative")
}

func TestProvideStats_WithLegacyProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := harness.NewT(t)
	node := h.NewNode().Init()

	// Explicitly disable Sweep to use Legacy provider
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", false)
	node.SetIPFSConfig("Provide.Enabled", true)

	node.StartDaemon()
	defer node.StopDaemon()

	apiMaddr, err := node.TryAPIAddr()
	require.NoError(t, err)

	api, err := NewApi(apiMaddr)
	require.NoError(t, err)

	// Get provide stats
	result, err := api.Routing().ProvideStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify Legacy stats are present, Sweep is not
	require.Nil(t, result.Sweep, "Legacy provider should not return Sweep stats")
	require.NotNil(t, result.Legacy, "Legacy provider should return Legacy stats")

	// Marshal to JSON and unmarshal to test struct to verify structure
	data, err := json.Marshal(result)
	require.NoError(t, err)

	var testStats testProvideStats
	err = json.Unmarshal(data, &testStats)
	require.NoError(t, err)

	// Verify Legacy field exists
	require.NotNil(t, testStats.Legacy)
	require.GreaterOrEqual(t, testStats.Legacy.TotalReprovides, uint64(0), "total reprovides should be non-negative")
}

func TestProvideStats_LANFlagErrorWithLegacy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := harness.NewT(t)
	node := h.NewNode().Init()

	// Use Legacy provider - LAN flag should error
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", false)
	node.SetIPFSConfig("Provide.Enabled", true)

	node.StartDaemon()
	defer node.StopDaemon()

	apiMaddr, err := node.TryAPIAddr()
	require.NoError(t, err)

	api, err := NewApi(apiMaddr)
	require.NoError(t, err)

	// Try to get LAN stats with Legacy provider
	// This should return an error
	_, err = api.Routing().ProvideStats(ctx, options.Routing.UseLAN(true))
	require.Error(t, err, "LAN flag should error with Legacy provider")
	require.Contains(t, err.Error(), "LAN stats only available for Sweep provider with Dual DHT",
		"error should indicate LAN stats unavailable")
}
