package autoconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAutoConfig(t *testing.T) {
	client := &Client{}

	t.Run("valid config passes validation", func(t *testing.T) {
		config := &AutoConfig{
			AutoConfigVersion: 123,
			Bootstrap: []string{
				"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
				"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest",
			},
			DNSResolvers: map[string][]string{
				"eth.": {"https://dns.example.com/dns-query"},
				"foo.": {"http://localhost:8080/dns-query", "https://1.2.3.4/dns-query"},
			},
			DelegatedRouters: map[string]DelegatedRouterConfig{
				"mainnet-for-nodes-with-dht": {
					"https://cid.contact/routing/v1/providers",
					"http://192.168.1.1:8080/routing/v1/providers",
				},
				"mainnet-for-nodes-without-dht": {
					"https://delegated-ipfs.dev/routing/v1/ipns",
				},
			},
			DelegatedPublishers: map[string]DelegatedPublisherConfig{
				"mainnet-for-ipns-publishers-with-http": {
					"https://delegated-ipfs.dev/routing/v1/ipns",
					"http://localhost:9090/routing/v1/ipns",
				},
			},
		}

		err := client.validateAutoConfig(config)
		assert.NoError(t, err)
	})

	t.Run("invalid bootstrap multiaddr fails validation", func(t *testing.T) {
		config := &AutoConfig{
			AutoConfigVersion: 123,
			Bootstrap: []string{
				"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
				"invalid-multiaddr",
			},
		}

		err := client.validateAutoConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Bootstrap[1] invalid multiaddr")
		assert.Contains(t, err.Error(), "invalid-multiaddr")
	})

	t.Run("invalid DNS resolver URL fails validation", func(t *testing.T) {
		config := &AutoConfig{
			AutoConfigVersion: 123,
			DNSResolvers: map[string][]string{
				"eth.": {"https://valid.example.com"},
				"bad.": {"://invalid-url"},
			},
		}

		err := client.validateAutoConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DNSResolvers[\"bad.\"][0] invalid URL")
		assert.Contains(t, err.Error(), "://invalid-url")
	})

	t.Run("invalid delegated router URL fails validation", func(t *testing.T) {
		config := &AutoConfig{
			AutoConfigVersion: 123,
			DelegatedRouters: map[string]DelegatedRouterConfig{
				"test": {
					"https://valid.example.com/routing/v1/providers",
					"://invalid-missing-scheme",
				},
			},
		}

		err := client.validateAutoConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DelegatedRouters[\"test\"][1] invalid URL")
		assert.Contains(t, err.Error(), "://invalid-missing-scheme")
	})

	t.Run("invalid delegated publisher URL fails validation", func(t *testing.T) {
		config := &AutoConfig{
			AutoConfigVersion: 123,
			DelegatedPublishers: map[string]DelegatedPublisherConfig{
				"test": {
					"https://valid.example.com/routing/v1/ipns",
					"ftp://unsupported-but-valid-url.com/routing/v1/ipns", // Should pass - we only check if URL parses
					"ht!@#$%^&*()tp://invalid-escape",                     // Should fail - invalid URL escape
				},
			},
		}

		err := client.validateAutoConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DelegatedPublishers[\"test\"][2] invalid URL")
		assert.Contains(t, err.Error(), "ht!@#$%^&*()tp://invalid-escape")
	})

	t.Run("empty config passes validation", func(t *testing.T) {
		config := &AutoConfig{
			AutoConfigVersion: 123,
		}

		err := client.validateAutoConfig(config)
		assert.NoError(t, err)
	})

	t.Run("various valid URL schemes are accepted", func(t *testing.T) {
		config := &AutoConfig{
			AutoConfigVersion: 123,
			DNSResolvers: map[string][]string{
				"test.": {
					"https://example.com",
					"http://localhost:8080",
					"http://192.168.1.1:9090",
					"https://1.2.3.4:443/path",
					"http://[::1]:8080/dns-query", // IPv6
				},
			},
		}

		err := client.validateAutoConfig(config)
		assert.NoError(t, err)
	})
}
