package config

import (
	"testing"
	"time"
)

func TestAutoConfigDefaults(t *testing.T) {
	// Test that AutoConfig has the correct default values
	cfg := &Config{
		AutoConfig: AutoConfig{
			URL:     DefaultAutoConfigURL,
			Enabled: True,
		},
	}

	if cfg.AutoConfig.URL != "https://config.ipfs-mainnet.org/autoconfig.json" {
		t.Errorf("expected AutoConfig.URL to be %s, got %s", DefaultAutoConfigURL, cfg.AutoConfig.URL)
	}

	if !cfg.AutoConfig.Enabled.WithDefault(true) {
		t.Error("expected AutoConfig.Enabled to be true by default")
	}

	// Test default check interval
	if cfg.AutoConfig.CheckInterval == nil {
		// This is expected - nil means use default
		duration := (*OptionalDuration)(nil).WithDefault(DefaultAutoConfigInterval)
		if duration != 24*time.Hour {
			t.Errorf("expected default check interval to be 24h, got %v", duration)
		}
	}
}

func TestAutoConfigProfile(t *testing.T) {
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
		AutoConfig: AutoConfig{
			Enabled: False,
		},
	}

	// Apply autoconfig profile
	profile, ok := Profiles["autoconfig"]
	if !ok {
		t.Fatal("autoconfig profile not found")
	}

	err := profile.Transform(cfg)
	if err != nil {
		t.Fatalf("failed to apply autoconfig profile: %v", err)
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

	// Check that AutoConfig was enabled
	if !cfg.AutoConfig.Enabled.WithDefault(true) {
		t.Error("expected AutoConfig.Enabled to be true after applying profile")
	}

	// Check that URL was set
	if cfg.AutoConfig.URL != DefaultAutoConfigURL {
		t.Errorf("expected AutoConfig.URL to be %s, got %s", DefaultAutoConfigURL, cfg.AutoConfig.URL)
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

	// Check that AutoConfig is enabled with correct URL
	if !cfg.AutoConfig.Enabled.WithDefault(true) {
		t.Error("expected AutoConfig.Enabled to be true")
	}

	if cfg.AutoConfig.URL != DefaultAutoConfigURL {
		t.Errorf("expected AutoConfig.URL to be %s, got %s", DefaultAutoConfigURL, cfg.AutoConfig.URL)
	}
}
