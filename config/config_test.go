package config

import (
	"testing"

	"github.com/ipfs/go-ipfs/repo/common"
	"github.com/stretchr/testify/require"
)

func TestClone(t *testing.T) {
	c := new(Config)
	c.Identity.PeerID = "faketest"
	c.API.HTTPHeaders = map[string][]string{"foo": {"bar"}}

	newCfg, err := c.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if newCfg.Identity.PeerID != c.Identity.PeerID {
		t.Fatal("peer ID not preserved")
	}

	c.API.HTTPHeaders["foo"] = []string{"baz"}
	if newCfg.API.HTTPHeaders["foo"][0] != "bar" {
		t.Fatal("HTTP headers not preserved")
	}

	delete(c.API.HTTPHeaders, "foo")
	if newCfg.API.HTTPHeaders["foo"][0] != "bar" {
		t.Fatal("HTTP headers not preserved")
	}
}

// We are testing something that overrides *existing* values in the defaults.
// Anything other than a map is overridden as is, a map is inspected more deeply
//  to override only set children.
func TestOverrideConfig(t *testing.T) {
	overrideConfig := func(c *Config, m map[string]interface{}) *Config {
		configMap, err := ToMap(c)
		require.NoError(t, err)
		OverrideMap(configMap, m)
		overridden, err := FromMap(configMap)
		require.NoError(t, err)
		return overridden
	}
	cloneConfig := func(c *Config) *Config {
		cloned, err := c.Clone()
		require.NoError(t, err)
		return cloned
	}
	MOD_TOKEN := "CONFIG-VALUE-MODIFIED-BY-TestOverrideMap"
	modKey := func(c *Config, k string) *Config {
		m, err := ToMap(c)
		require.NoError(t, err)
		require.NoError(t, common.MapSetKV(m, k, MOD_TOKEN))
		modConfig, err := FromMap(m)
		require.NoError(t, err)
		return modConfig
	}

	testOverrides := func(c *Config) {
		require.Equal(t, c, cloneConfig(c))
		require.Equal(t, c, overrideConfig(c, nil))
		require.Equal(t, modKey(c, "Identity.PeerID"),
			overrideConfig(c, map[string]interface{}{
				"Identity": map[string]interface{}{
					"PeerID": MOD_TOKEN,
				},
			}))
		require.Equal(t, modKey(c, "Datastore.StorageMax"),
			overrideConfig(c, map[string]interface{}{
				"Datastore": map[string]interface{}{
					"StorageMax": MOD_TOKEN,
				},
			}))
	}

	testOverrides(&Config{})
	defaultConfig, err := DefaultConfig("")
	require.NoError(t, err)
	testOverrides(defaultConfig)
}
