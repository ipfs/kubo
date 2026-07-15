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

// stubHost is a minimal host.Host stub for testing httpRouterAddrFunc.
// hostAddrs mocks Addrs(), which in a real host returns wildcard-resolved
// interface addresses after the AddrsFactory ran (NoAnnounce filtering, the
// AutoTLS /tls/ws address, certhashes).
type stubHost struct {
	hostAddrs []ma.Multiaddr
	reachable []ma.Multiaddr
}

// ConfirmedAddrs mocks the AutoNAT V2 confirmed set. httpRouterAddrFunc must
// not consult it: AutoNAT only sees listen addresses, so the AddrsFactory-
// synthesized AutoTLS /tls/ws address can never appear in it, and a record
// narrowed to this set carries nothing a browser can dial.
// See https://github.com/ipfs/kubo/issues/11369.
func (h *stubHost) ConfirmedAddrs() (reachable, unreachable, unknown []ma.Multiaddr) {
	return h.reachable, nil, nil
}

func (h *stubHost) ID() peer.ID                                         { panic("unused") }
func (h *stubHost) Addrs() []ma.Multiaddr                               { return h.hostAddrs }
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
	// A publicly reachable node with every transport kubo can announce.
	// This is what host.Addrs() returns in a running daemon: wildcard Swarm
	// binds resolved to concrete interfaces (so the LAN address shows up
	// next to the public one), AddrsFactory applied, certhashes attached.
	const (
		autoTLSWS    = "/dns4/1-2-3-4.k51qzi5uqu5dlwbcbb2u3ptbnhs5lpj4nc3xbhrgqrkgo3l0h5xgv9k1nn9k1.libp2p.direct/tcp/4001/tls/ws"
		webRTCDirect = "/ip4/1.2.3.4/udp/4001/webrtc-direct/certhash/uEiDDq4_xKlbjmeSaEwB7UUcH_TS1z2WLwkGuOHZgKZgUSw"
		webTransport = "/ip4/1.2.3.4/udp/4001/quic-v1/webtransport/certhash/uEiDDq4_xKlbjmeSaEwB7UUcH_TS1z2WLwkGuOHZgKZgUSw"
		publicTCP    = "/ip4/1.2.3.4/tcp/4001"
		publicQUIC   = "/ip4/1.2.3.4/udp/4001/quic-v1"
		lanTCP       = "/ip4/192.168.1.10/tcp/4001"
		lanQUIC      = "/ip4/192.168.1.10/udp/4001/quic-v1"
		loopbackTCP  = "/ip4/127.0.0.1/tcp/4001"
	)
	publicNodeAddrs := []string{autoTLSWS, webRTCDirect, webTransport, publicTCP, publicQUIC, lanTCP, loopbackTCP}
	lanOnlyAddrs := []string{lanTCP, lanQUIC}

	tests := []struct {
		name      string
		hostAddrs []string // host.Addrs() output (nil = none)
		reachable []string // AutoNAT V2 confirmed set; must NOT influence the result
		cfg       config.Addresses
		want      []string
	}{
		{
			// Regression test for https://github.com/ipfs/kubo/issues/11369:
			// the record must be built from host.Addrs(), never from
			// ConfirmedAddrs. reachable carries the narrowed set AutoNAT
			// reports on such a node (no AutoTLS, no webrtc-direct); the
			// browser-dialable addrs from hostAddrs must win.
			name:      "browser-dialable transports reach the record on a public node",
			hostAddrs: publicNodeAddrs,
			reachable: []string{publicTCP, publicQUIC, webTransport},
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"}},
			want:      []string{autoTLSWS, webRTCDirect, webTransport, publicTCP, publicQUIC},
		},
		{
			name:      "LAN-only node announces what it has",
			hostAddrs: lanOnlyAddrs,
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"}},
			want:      lanOnlyAddrs,
		},
		{
			name:      "empty host.Addrs announces nothing",
			hostAddrs: nil,
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}},
			want:      nil,
		},
		{
			name:      "empty host.Addrs with AppendAnnounce announces AppendAnnounce",
			hostAddrs: nil,
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}, AppendAnnounce: []string{"/ip4/10.0.0.1/tcp/4001"}},
			want:      []string{"/ip4/10.0.0.1/tcp/4001"},
		},
		{
			name:      "Announce overrides host.Addrs",
			hostAddrs: publicNodeAddrs,
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}, Announce: []string{"/ip4/5.6.7.8/tcp/4001"}},
			want:      []string{"/ip4/5.6.7.8/tcp/4001"},
		},
		{
			// AppendAnnounce is an explicit operator override, so it survives
			// the public-address filter even when it is a LAN address.
			name:      "AppendAnnounce added to host.Addrs",
			hostAddrs: []string{publicTCP, lanTCP},
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}, AppendAnnounce: []string{"/ip4/10.0.0.1/tcp/4001"}},
			want:      []string{publicTCP, "/ip4/10.0.0.1/tcp/4001"},
		},
		{
			name:      "AppendAnnounce added to LAN-only fallback",
			hostAddrs: lanOnlyAddrs,
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}, AppendAnnounce: []string{"/ip4/10.0.0.1/tcp/4001"}},
			want:      append(append([]string{}, lanOnlyAddrs...), "/ip4/10.0.0.1/tcp/4001"),
		},
		{
			// The AddrsFactory injects AppendAnnounce into h.Addrs(), so the
			// same addr arrives twice: once inside hostAddrs and once via the
			// explicit re-append. The record must carry it exactly once.
			name:      "AppendAnnounce present in host.Addrs is not duplicated",
			hostAddrs: []string{publicTCP, "/ip4/5.6.7.8/tcp/4001"},
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}, AppendAnnounce: []string{"/ip4/5.6.7.8/tcp/4001"}},
			want:      []string{publicTCP, "/ip4/5.6.7.8/tcp/4001"},
		},
		{
			// A public AppendAnnounce echoed back through h.Addrs() must not
			// count as "the node has a public address" and evict the real
			// LAN addresses of an otherwise private node.
			name:      "public AppendAnnounce does not evict LAN-only fallback addrs",
			hostAddrs: append([]string{"/ip4/5.6.7.8/tcp/4001"}, lanOnlyAddrs...),
			cfg:       config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}, AppendAnnounce: []string{"/ip4/5.6.7.8/tcp/4001"}},
			want:      append(append([]string{}, lanOnlyAddrs...), "/ip4/5.6.7.8/tcp/4001"),
		},
		{
			// NoAnnounce (including server profile CIDR ranges) is applied by the
			// libp2p AddrsFactory before host.Addrs() returns, so httpRouterAddrFunc
			// never sees the filtered addresses.
			name:      "NoAnnounce filtering happens upstream in host.Addrs",
			hostAddrs: []string{lanTCP}, // already filtered by addrFactory
			cfg: config.Addresses{
				Swarm:      []string{"/ip4/0.0.0.0/tcp/4001"},
				NoAnnounce: []string{"/ip4/127.0.0.0/ipcidr/8"},
			},
			want: []string{lanTCP},
		},
		{
			name: "AppendAnnounce added to Announce",
			cfg:  config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}, Announce: []string{"/ip4/5.6.7.8/tcp/4001"}, AppendAnnounce: []string{"/ip4/10.0.0.1/tcp/4001"}},
			want: []string{"/ip4/5.6.7.8/tcp/4001", "/ip4/10.0.0.1/tcp/4001"},
		},
		{
			name: "AppendAnnounce overlapping Announce is not duplicated",
			cfg:  config.Addresses{Swarm: []string{"/ip4/0.0.0.0/tcp/4001"}, Announce: []string{"/ip4/5.6.7.8/tcp/4001"}, AppendAnnounce: []string{"/ip4/5.6.7.8/tcp/4001", "/ip4/10.0.0.1/tcp/4001"}},
			want: []string{"/ip4/5.6.7.8/tcp/4001", "/ip4/10.0.0.1/tcp/4001"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &stubHost{
				hostAddrs: parseMultiaddrs(tt.hostAddrs),
				reachable: parseMultiaddrs(tt.reachable),
			}
			fn := httpRouterAddrFunc(h, tt.cfg)
			assert.Equal(t, parseMultiaddrs(tt.want), fn())
		})
	}
}
