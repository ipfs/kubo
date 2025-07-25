package libp2p

import (
	"testing"

	config "github.com/ipfs/kubo/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpAddrsFromConfig(t *testing.T) {
	require.Equal(t, []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
		httpAddrsFromConfig(config.Addresses{
			Swarm: []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
		}), "Swarm addrs should be taken by default")

	require.Equal(t, []string{"/ip4/192.168.0.1/tcp/4001"},
		httpAddrsFromConfig(config.Addresses{
			Swarm:    []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
			Announce: []string{"/ip4/192.168.0.1/tcp/4001"},
		}), "Announce addrs should override Swarm if specified")

	require.Equal(t, []string{"/ip4/0.0.0.0/udp/4001/quic-v1"},
		httpAddrsFromConfig(config.Addresses{
			Swarm:      []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
			NoAnnounce: []string{"/ip4/0.0.0.0/tcp/4001"},
		}), "Swarm addrs should not contain NoAnnounce addrs")

	require.Equal(t, []string{"/ip4/192.168.0.1/tcp/4001", "/ip4/192.168.0.2/tcp/4001"},
		httpAddrsFromConfig(config.Addresses{
			Swarm:          []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
			Announce:       []string{"/ip4/192.168.0.1/tcp/4001"},
			AppendAnnounce: []string{"/ip4/192.168.0.2/tcp/4001"},
		}), "AppendAnnounce addrs should be included if specified")
}

func TestParseEndpointPath(t *testing.T) {
	tests := []struct {
		name                 string
		endpoint             string
		expectedBaseURL      string
		expectedCapabilities EndpointCapabilities
		expectError          bool
	}{
		{
			name:            "URL with no path should have All capability",
			endpoint:        "https://example.com",
			expectedBaseURL: "https://example.com",
			expectedCapabilities: EndpointCapabilities{
				All:       true,
				Providers: true,
				Peers:     true,
				IPNS:      true,
			},
			expectError: false,
		},
		{
			name:            "URL with trailing slash should have All capability",
			endpoint:        "https://example.com/",
			expectedBaseURL: "https://example.com",
			expectedCapabilities: EndpointCapabilities{
				All:       true,
				Providers: true,
				Peers:     true,
				IPNS:      true,
			},
			expectError: false,
		},
		{
			name:            "URL with IPNS path should have only IPNS capability",
			endpoint:        "https://example.com/routing/v1/ipns",
			expectedBaseURL: "https://example.com",
			expectedCapabilities: EndpointCapabilities{
				All:       false,
				Providers: false,
				Peers:     false,
				IPNS:      true,
			},
			expectError: false,
		},
		{
			name:            "URL with providers path should have only Providers capability",
			endpoint:        "https://example.com/routing/v1/providers",
			expectedBaseURL: "https://example.com",
			expectedCapabilities: EndpointCapabilities{
				All:       false,
				Providers: true,
				Peers:     false,
				IPNS:      false,
			},
			expectError: false,
		},
		{
			name:            "URL with peers path should have only Peers capability",
			endpoint:        "https://example.com/routing/v1/peers",
			expectedBaseURL: "https://example.com",
			expectedCapabilities: EndpointCapabilities{
				All:       false,
				Providers: false,
				Peers:     true,
				IPNS:      false,
			},
			expectError: false,
		},
		{
			name:        "URL with unsupported routing path should error",
			endpoint:    "https://example.com/routing/v1/unsupported",
			expectError: true,
		},
		{
			name:        "URL with invalid path should error",
			endpoint:    "https://example.com/invalid/path",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL, capabilities, err := parseEndpointPath(tt.endpoint)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedBaseURL, baseURL)
			assert.Equal(t, tt.expectedCapabilities, capabilities)
		})
	}
}

func TestEndpointCapabilitiesAllLogic(t *testing.T) {
	t.Run("All capability should enable individual capabilities", func(t *testing.T) {
		_, capabilities, err := parseEndpointPath("https://example.com")
		require.NoError(t, err)

		// When All is true, all individual capabilities should also be true
		assert.True(t, capabilities.All)
		assert.True(t, capabilities.Providers)
		assert.True(t, capabilities.Peers)
		assert.True(t, capabilities.IPNS)
	})

	t.Run("Specific path should disable All capability", func(t *testing.T) {
		_, capabilities, err := parseEndpointPath("https://example.com/routing/v1/ipns")
		require.NoError(t, err)

		// When a specific path is used, All should be false
		assert.False(t, capabilities.All)
		assert.False(t, capabilities.Providers)
		assert.False(t, capabilities.Peers)
		assert.True(t, capabilities.IPNS)
	})
}
