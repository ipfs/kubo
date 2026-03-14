package libp2p

import (
	"context"
	"testing"

	"github.com/ipfs/boxo/autoconf"
	config "github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
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

func TestDetermineCapabilities(t *testing.T) {
	tests := []struct {
		name                 string
		endpoint             EndpointSource
		expectedBaseURL      string
		expectedCapabilities autoconf.EndpointCapabilities
		expectError          bool
	}{
		{
			name: "URL with no path should have all Read capabilities",
			endpoint: EndpointSource{
				URL:           "https://example.com",
				SupportsRead:  true,
				SupportsWrite: false,
			},
			expectedBaseURL: "https://example.com",
			expectedCapabilities: autoconf.EndpointCapabilities{
				Providers: true,
				Peers:     true,
				IPNSGet:   true,
				IPNSPut:   false,
			},
			expectError: false,
		},
		{
			name: "URL with trailing slash should have all Read capabilities",
			endpoint: EndpointSource{
				URL:           "https://example.com/",
				SupportsRead:  true,
				SupportsWrite: false,
			},
			expectedBaseURL: "https://example.com",
			expectedCapabilities: autoconf.EndpointCapabilities{
				Providers: true,
				Peers:     true,
				IPNSGet:   true,
				IPNSPut:   false,
			},
			expectError: false,
		},
		{
			name: "URL with IPNS path should have only IPNS capabilities",
			endpoint: EndpointSource{
				URL:           "https://example.com/routing/v1/ipns",
				SupportsRead:  true,
				SupportsWrite: true,
			},
			expectedBaseURL: "https://example.com",
			expectedCapabilities: autoconf.EndpointCapabilities{
				Providers: false,
				Peers:     false,
				IPNSGet:   true,
				IPNSPut:   true,
			},
			expectError: false,
		},
		{
			name: "URL with providers path should have only Providers capability",
			endpoint: EndpointSource{
				URL:           "https://example.com/routing/v1/providers",
				SupportsRead:  true,
				SupportsWrite: false,
			},
			expectedBaseURL: "https://example.com",
			expectedCapabilities: autoconf.EndpointCapabilities{
				Providers: true,
				Peers:     false,
				IPNSGet:   false,
				IPNSPut:   false,
			},
			expectError: false,
		},
		{
			name: "URL with peers path should have only Peers capability",
			endpoint: EndpointSource{
				URL:           "https://example.com/routing/v1/peers",
				SupportsRead:  true,
				SupportsWrite: false,
			},
			expectedBaseURL: "https://example.com",
			expectedCapabilities: autoconf.EndpointCapabilities{
				Providers: false,
				Peers:     true,
				IPNSGet:   false,
				IPNSPut:   false,
			},
			expectError: false,
		},
		{
			name: "URL with Write support only should enable IPNSPut for no-path endpoint",
			endpoint: EndpointSource{
				URL:           "https://example.com",
				SupportsRead:  false,
				SupportsWrite: true,
			},
			expectedBaseURL: "https://example.com",
			expectedCapabilities: autoconf.EndpointCapabilities{
				Providers: false,
				Peers:     false,
				IPNSGet:   false,
				IPNSPut:   true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL, capabilities, err := determineCapabilities(tt.endpoint)

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

func TestEndpointCapabilitiesReadWriteLogic(t *testing.T) {
	t.Run("Read endpoint with no path should enable read capabilities", func(t *testing.T) {
		endpoint := EndpointSource{
			URL:           "https://example.com",
			SupportsRead:  true,
			SupportsWrite: false,
		}
		_, capabilities, err := determineCapabilities(endpoint)
		require.NoError(t, err)

		// Read endpoint with no path should enable all read capabilities
		assert.True(t, capabilities.Providers)
		assert.True(t, capabilities.Peers)
		assert.True(t, capabilities.IPNSGet)
		assert.False(t, capabilities.IPNSPut) // Write capability should be false
	})

	t.Run("Write endpoint with no path should enable write capabilities", func(t *testing.T) {
		endpoint := EndpointSource{
			URL:           "https://example.com",
			SupportsRead:  false,
			SupportsWrite: true,
		}
		_, capabilities, err := determineCapabilities(endpoint)
		require.NoError(t, err)

		// Write endpoint with no path should only enable IPNS write capability
		assert.False(t, capabilities.Providers)
		assert.False(t, capabilities.Peers)
		assert.False(t, capabilities.IPNSGet)
		assert.True(t, capabilities.IPNSPut) // Only write capability should be true
	})

	t.Run("Specific path should only enable matching capabilities", func(t *testing.T) {
		endpoint := EndpointSource{
			URL:           "https://example.com/routing/v1/ipns",
			SupportsRead:  true,
			SupportsWrite: true,
		}
		_, capabilities, err := determineCapabilities(endpoint)
		require.NoError(t, err)

		// Specific IPNS path should only enable IPNS capabilities based on source
		assert.False(t, capabilities.Providers)
		assert.False(t, capabilities.Peers)
		assert.True(t, capabilities.IPNSGet) // Read capability enabled
		assert.True(t, capabilities.IPNSPut) // Write capability enabled
	})

	t.Run("Unsupported paths should result in empty capabilities", func(t *testing.T) {
		endpoint := EndpointSource{
			URL:           "https://example.com/routing/v1/unsupported",
			SupportsRead:  true,
			SupportsWrite: false,
		}
		_, capabilities, err := determineCapabilities(endpoint)
		require.NoError(t, err)

		// Unsupported paths should result in no capabilities
		assert.False(t, capabilities.Providers)
		assert.False(t, capabilities.Peers)
		assert.False(t, capabilities.IPNSGet)
		assert.False(t, capabilities.IPNSPut)
	})
}

func mustMultiaddr(s string) ma.Multiaddr {
	a, err := ma.NewMultiaddr(s)
	if err != nil {
		panic(err)
	}
	return a
}

// stubHost is a minimal host.Host stub for testing httpRouterAddrFunc.
// Only the methods checked via type assertion (confirmedAddrsHost) matter;
// all other methods panic if called.
type stubHost struct {
	reachable []ma.Multiaddr
}

func (h *stubHost) ConfirmedAddrs() (reachable, unreachable, unknown []ma.Multiaddr) {
	return h.reachable, nil, nil
}

func (h *stubHost) ID() peer.ID                                         { panic("unused") }
func (h *stubHost) Addrs() []ma.Multiaddr                               { panic("unused") }
func (h *stubHost) Peerstore() peerstore.Peerstore                      { panic("unused") }
func (h *stubHost) Network() network.Network                            { panic("unused") }
func (h *stubHost) Mux() protocol.Switch                                { panic("unused") }
func (h *stubHost) Connect(context.Context, peer.AddrInfo) error        { panic("unused") }
func (h *stubHost) SetStreamHandler(protocol.ID, network.StreamHandler) { panic("unused") }
func (h *stubHost) SetStreamHandlerMatch(protocol.ID, func(protocol.ID) bool, network.StreamHandler) {
	panic("unused")
}
func (h *stubHost) RemoveStreamHandler(protocol.ID) { panic("unused") }
func (h *stubHost) NewStream(context.Context, peer.ID, ...protocol.ID) (network.Stream, error) {
	panic("unused")
}
func (h *stubHost) Close() error                     { panic("unused") }
func (h *stubHost) ConnManager() connmgr.ConnManager { panic("unused") }
func (h *stubHost) EventBus() event.Bus              { panic("unused") }

func TestHttpRouterAddrFunc(t *testing.T) {
	t.Run("prefers autonat confirmed reachable addrs over swarm fallback", func(t *testing.T) {
		h := &stubHost{
			reachable: []ma.Multiaddr{
				mustMultiaddr("/ip4/1.2.3.4/tcp/4001"),
				mustMultiaddr("/ip4/1.2.3.4/udp/4001/quic-v1"),
			},
		}
		fn := httpRouterAddrFunc(h, config.Addresses{
			Swarm: []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
		})
		assert.Equal(t, h.reachable, fn())
	})

	t.Run("falls back to swarm when autonat has no confirmed addrs", func(t *testing.T) {
		h := &stubHost{reachable: nil}
		fn := httpRouterAddrFunc(h, config.Addresses{
			Swarm: []string{"/ip4/0.0.0.0/tcp/4001"},
		})
		assert.Equal(t, []ma.Multiaddr{mustMultiaddr("/ip4/0.0.0.0/tcp/4001")}, fn())
	})

	t.Run("Announce overrides autonat and swarm", func(t *testing.T) {
		h := &stubHost{
			reachable: []ma.Multiaddr{mustMultiaddr("/ip4/1.2.3.4/tcp/4001")},
		}
		fn := httpRouterAddrFunc(h, config.Addresses{
			Swarm:    []string{"/ip4/0.0.0.0/tcp/4001"},
			Announce: []string{"/ip4/5.6.7.8/tcp/4001"},
		})
		assert.Equal(t, []ma.Multiaddr{mustMultiaddr("/ip4/5.6.7.8/tcp/4001")}, fn())
	})

	t.Run("AppendAnnounce added to autonat addrs", func(t *testing.T) {
		h := &stubHost{
			reachable: []ma.Multiaddr{mustMultiaddr("/ip4/1.2.3.4/tcp/4001")},
		}
		fn := httpRouterAddrFunc(h, config.Addresses{
			Swarm:          []string{"/ip4/0.0.0.0/tcp/4001"},
			AppendAnnounce: []string{"/ip4/10.0.0.1/tcp/4001"},
		})
		assert.Equal(t, []ma.Multiaddr{
			mustMultiaddr("/ip4/1.2.3.4/tcp/4001"),
			mustMultiaddr("/ip4/10.0.0.1/tcp/4001"),
		}, fn())
	})

	t.Run("AppendAnnounce added to swarm fallback", func(t *testing.T) {
		h := &stubHost{reachable: nil}
		fn := httpRouterAddrFunc(h, config.Addresses{
			Swarm:          []string{"/ip4/0.0.0.0/tcp/4001"},
			AppendAnnounce: []string{"/ip4/10.0.0.1/tcp/4001"},
		})
		assert.Equal(t, []ma.Multiaddr{
			mustMultiaddr("/ip4/0.0.0.0/tcp/4001"),
			mustMultiaddr("/ip4/10.0.0.1/tcp/4001"),
		}, fn())
	})

	t.Run("NoAnnounce filters swarm fallback", func(t *testing.T) {
		h := &stubHost{reachable: nil}
		fn := httpRouterAddrFunc(h, config.Addresses{
			Swarm:      []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
			NoAnnounce: []string{"/ip4/0.0.0.0/tcp/4001"},
		})
		assert.Equal(t, []ma.Multiaddr{mustMultiaddr("/ip4/0.0.0.0/udp/4001/quic-v1")}, fn())
	})

	t.Run("Announce is not combined with AppendAnnounce", func(t *testing.T) {
		h := &stubHost{reachable: nil}
		fn := httpRouterAddrFunc(h, config.Addresses{
			Swarm:          []string{"/ip4/0.0.0.0/tcp/4001"},
			Announce:       []string{"/ip4/5.6.7.8/tcp/4001"},
			AppendAnnounce: []string{"/ip4/10.0.0.1/tcp/4001"},
		})
		// Announce is a full override; AppendAnnounce is ignored.
		assert.Equal(t, []ma.Multiaddr{mustMultiaddr("/ip4/5.6.7.8/tcp/4001")}, fn())
	})
}
