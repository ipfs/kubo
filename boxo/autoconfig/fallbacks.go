package autoconfig

// Fallback defaults matching Kubo 0.36
// These are used as last-resort fallback when autoconfig fetch fails and no cache exists
var (
	// FallbackBootstrapPeers are the default bootstrap peers from Kubo 0.36
	// Used as last-resort fallback when autoconfig fetch fails
	FallbackBootstrapPeers = []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/dnsaddr/va1.bootstrap.libp2p.io/p2p/12D3KooWKnDdG3iXw9eTFijk3EWSunZcFi54Zka4wmtqtt6rPxc8",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.131.131.82/udp/4001/quic-v1/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}

	// FallbackDNSResolvers are the default DNS resolvers matching mainnet autoconfig
	// Used as last-resort fallback when autoconfig fetch fails
	// Only "eth." has explicit fallbacks - no "." (root domain) fallback is intentional
	// to ensure users' OS DNS resolver is used by default, allowing browser clients
	// to make their own DoH decisions based on privacy preferences
	FallbackDNSResolvers = map[string][]string{
		"eth.": {
			"https://dns.eth.limo/dns-query",
			"https://dns.eth.link/dns-query",
		},
	}

	// FallbackDelegatedRouters are the default delegated routing endpoints from Kubo 0.36
	// Used as last-resort fallback when autoconfig fetch fails
	FallbackDelegatedRouters = []string{
		"https://cid.contact/routing/v1/providers",
	}

	// FallbackDelegatedPublishers are the default delegated IPNS publishers matching mainnet autoconfig
	// Used as last-resort fallback when autoconfig fetch fails
	FallbackDelegatedPublishers = []string{
		"https://delegated-ipfs.dev/routing/v1/ipns",
	}
)
