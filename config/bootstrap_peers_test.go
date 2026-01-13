package config

import (
	"testing"

	"github.com/ipfs/boxo/autoconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapPeerStrings(t *testing.T) {
	// Test round-trip: string -> parse -> format -> string
	// This ensures that parsing and formatting are inverse operations

	// Start with the default bootstrap peer multiaddr strings
	originalStrings := autoconf.FallbackBootstrapPeers

	// Parse multiaddr strings into structured peer data
	parsed, err := ParseBootstrapPeers(originalStrings)
	require.NoError(t, err, "parsing bootstrap peers should succeed")

	// Format the parsed data back into multiaddr strings
	formattedStrings := BootstrapPeerStrings(parsed)

	// Verify round-trip: we should get back exactly what we started with
	assert.ElementsMatch(t, originalStrings, formattedStrings,
		"round-trip through parse/format should preserve all bootstrap peers")
}
