package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoConfDefaults(t *testing.T) {
	// Test that AutoConf has the correct default values
	cfg := &Config{
		AutoConf: AutoConf{
			URL:     NewOptionalString(DefaultAutoConfURL),
			Enabled: True,
		},
	}

	assert.Equal(t, DefaultAutoConfURL, cfg.AutoConf.URL.WithDefault(DefaultAutoConfURL))
	assert.True(t, cfg.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled))

	// Test default refresh interval
	if cfg.AutoConf.RefreshInterval == nil {
		// This is expected - nil means use default
		duration := (*OptionalDuration)(nil).WithDefault(DefaultAutoConfRefreshInterval)
		assert.Equal(t, DefaultAutoConfRefreshInterval, duration)
	}
}

func TestAutoConfProfile(t *testing.T) {
	cfg := &Config{
		Bootstrap: []string{"some", "existing", "peers"},
		DNS: DNS{
			Resolvers: map[string]string{
				"eth.": "https://example.com",
			},
		},
		Routing: Routing{
			DelegatedRouters: []string{"https://existing.router"},
		},
		Ipns: Ipns{
			DelegatedPublishers: []string{"https://existing.publisher"},
		},
		AutoConf: AutoConf{
			Enabled: False,
		},
	}

	// Apply autoconf profile
	profile, ok := Profiles["autoconf-on"]
	require.True(t, ok, "autoconf-on profile not found")

	err := profile.Transform(cfg)
	require.NoError(t, err)

	// Check that values were set to "auto"
	assert.Equal(t, []string{AutoPlaceholder}, cfg.Bootstrap)
	assert.Equal(t, AutoPlaceholder, cfg.DNS.Resolvers["."])
	assert.Equal(t, []string{AutoPlaceholder}, cfg.Routing.DelegatedRouters)
	assert.Equal(t, []string{AutoPlaceholder}, cfg.Ipns.DelegatedPublishers)

	// Check that AutoConf was enabled
	assert.True(t, cfg.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled))

	// Check that URL was set
	assert.Equal(t, DefaultAutoConfURL, cfg.AutoConf.URL.WithDefault(DefaultAutoConfURL))
}

func TestInitWithAutoValues(t *testing.T) {
	identity := Identity{
		PeerID: "QmTest",
	}

	cfg, err := InitWithIdentity(identity)
	require.NoError(t, err)

	// Check that Bootstrap is set to "auto"
	assert.Equal(t, []string{AutoPlaceholder}, cfg.Bootstrap)

	// Check that DNS resolver is set to "auto"
	assert.Equal(t, AutoPlaceholder, cfg.DNS.Resolvers["."])

	// Check that DelegatedRouters is set to "auto"
	assert.Equal(t, []string{AutoPlaceholder}, cfg.Routing.DelegatedRouters)

	// Check that DelegatedPublishers is set to "auto"
	assert.Equal(t, []string{AutoPlaceholder}, cfg.Ipns.DelegatedPublishers)

	// Check that AutoConf is enabled with correct URL
	assert.True(t, cfg.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled))
	assert.Equal(t, DefaultAutoConfURL, cfg.AutoConf.URL.WithDefault(DefaultAutoConfURL))
}
