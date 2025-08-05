package config

import (
	"testing"
)

func TestAutoConfDefaults(t *testing.T) {
	// Test that AutoConf has the correct default values
	cfg := &Config{
		AutoConf: AutoConf{
			URL:     DefaultAutoConfURL,
			Enabled: True,
		},
	}

	if cfg.AutoConf.URL != DefaultAutoConfURL {
		t.Errorf("expected AutoConf.URL to be %s, got %s", DefaultAutoConfURL, cfg.AutoConf.URL)
	}

	if !cfg.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled) {
		t.Error("expected AutoConf.Enabled to be true by default")
	}

	// Test default refresh interval
	if cfg.AutoConf.RefreshInterval == nil {
		// This is expected - nil means use default
		duration := (*OptionalDuration)(nil).WithDefault(DefaultAutoConfRefreshInterval)
		if duration != DefaultAutoConfRefreshInterval {
			t.Errorf("expected default refresh interval to be 24h, got %v", duration)
		}
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
	if !ok {
		t.Fatal("autoconf-on profile not found")
	}

	err := profile.Transform(cfg)
	if err != nil {
		t.Fatalf("failed to apply autoconf profile: %v", err)
	}

	// Check that values were set to "auto"
	if len(cfg.Bootstrap) != 1 || cfg.Bootstrap[0] != AutoPlaceholder {
		t.Errorf("expected Bootstrap to be [%s], got %v", AutoPlaceholder, cfg.Bootstrap)
	}

	if cfg.DNS.Resolvers["."] != AutoPlaceholder {
		t.Errorf("expected DNS.Resolvers[\".\"] to be %s, got %s", AutoPlaceholder, cfg.DNS.Resolvers["."])
	}

	if len(cfg.Routing.DelegatedRouters) != 1 || cfg.Routing.DelegatedRouters[0] != AutoPlaceholder {
		t.Errorf("expected DelegatedRouters to be [%s], got %v", AutoPlaceholder, cfg.Routing.DelegatedRouters)
	}

	if len(cfg.Ipns.DelegatedPublishers) != 1 || cfg.Ipns.DelegatedPublishers[0] != AutoPlaceholder {
		t.Errorf("expected DelegatedPublishers to be [%s], got %v", AutoPlaceholder, cfg.Ipns.DelegatedPublishers)
	}

	// Check that AutoConf was enabled
	if !cfg.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled) {
		t.Error("expected AutoConf.Enabled to be true after applying profile")
	}

	// Check that URL was set
	if cfg.AutoConf.URL != DefaultAutoConfURL {
		t.Errorf("expected AutoConf.URL to be %s, got %s", DefaultAutoConfURL, cfg.AutoConf.URL)
	}
}

func TestInitWithAutoValues(t *testing.T) {
	identity := Identity{
		PeerID: "QmTest",
	}

	cfg, err := InitWithIdentity(identity)
	if err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	// Check that Bootstrap is set to "auto"
	if len(cfg.Bootstrap) != 1 || cfg.Bootstrap[0] != AutoPlaceholder {
		t.Errorf("expected Bootstrap to be [%s], got %v", AutoPlaceholder, cfg.Bootstrap)
	}

	// Check that DNS resolver is set to "auto"
	if cfg.DNS.Resolvers["."] != AutoPlaceholder {
		t.Errorf("expected DNS.Resolvers[\".\"] to be %s, got %s", AutoPlaceholder, cfg.DNS.Resolvers["."])
	}

	// Check that DelegatedRouters is set to "auto"
	if len(cfg.Routing.DelegatedRouters) != 1 || cfg.Routing.DelegatedRouters[0] != AutoPlaceholder {
		t.Errorf("expected DelegatedRouters to be [%s], got %v", AutoPlaceholder, cfg.Routing.DelegatedRouters)
	}

	// Check that DelegatedPublishers is set to "auto"
	if len(cfg.Ipns.DelegatedPublishers) != 1 || cfg.Ipns.DelegatedPublishers[0] != AutoPlaceholder {
		t.Errorf("expected DelegatedPublishers to be [%s], got %v", AutoPlaceholder, cfg.Ipns.DelegatedPublishers)
	}

	// Check that AutoConf is enabled with correct URL
	if !cfg.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled) {
		t.Error("expected AutoConf.Enabled to be true")
	}

	if cfg.AutoConf.URL != DefaultAutoConfURL {
		t.Errorf("expected AutoConf.URL to be %s, got %s", DefaultAutoConfURL, cfg.AutoConf.URL)
	}
}
