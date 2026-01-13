package config

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/ipfs/boxo/autoconf"
	logging "github.com/ipfs/go-log/v2"
	version "github.com/ipfs/kubo"
)

var autoconfLog = logging.Logger("autoconf")

// Singleton state for autoconf client
var (
	clientOnce  sync.Once
	clientCache *autoconf.Client
	clientErr   error
)

// GetAutoConfClient returns a cached autoconf client or creates a new one.
// This is thread-safe and uses a singleton pattern.
func GetAutoConfClient(cfg *Config) (*autoconf.Client, error) {
	clientOnce.Do(func() {
		clientCache, clientErr = newAutoConfClient(cfg)
	})
	return clientCache, clientErr
}

// newAutoConfClient creates a new autoconf client with the given config
func newAutoConfClient(cfg *Config) (*autoconf.Client, error) {
	// Get repo path for cache directory
	repoPath, err := PathRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo path: %w", err)
	}

	// Prepare refresh interval with nil check
	refreshInterval := cfg.AutoConf.RefreshInterval
	if refreshInterval == nil {
		refreshInterval = &OptionalDuration{}
	}

	// Use default URL if not specified
	url := cfg.AutoConf.URL.WithDefault(DefaultAutoConfURL)

	// Build client options
	options := []autoconf.Option{
		autoconf.WithCacheDir(filepath.Join(repoPath, "autoconf")),
		autoconf.WithUserAgent(version.GetUserAgentVersion()),
		autoconf.WithCacheSize(DefaultAutoConfCacheSize),
		autoconf.WithTimeout(DefaultAutoConfTimeout),
		autoconf.WithRefreshInterval(refreshInterval.WithDefault(DefaultAutoConfRefreshInterval)),
		autoconf.WithFallback(autoconf.GetMainnetFallbackConfig),
		autoconf.WithURL(url),
	}

	return autoconf.NewClient(options...)
}

// ValidateAutoConfWithRepo validates that autoconf setup is correct at daemon startup with repo access
func ValidateAutoConfWithRepo(cfg *Config, swarmKeyExists bool) error {
	if !cfg.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled) {
		// AutoConf is disabled, check for "auto" values and warn
		return validateAutoConfDisabled(cfg)
	}

	// Check for private network with default mainnet URL
	url := cfg.AutoConf.URL.WithDefault(DefaultAutoConfURL)
	if swarmKeyExists && url == DefaultAutoConfURL {
		return fmt.Errorf("AutoConf cannot use the default mainnet URL (%s) on a private network (swarm.key or LIBP2P_FORCE_PNET detected). Either disable AutoConf by setting AutoConf.Enabled=false, or configure AutoConf.URL to point to a configuration service specific to your private swarm", DefaultAutoConfURL)
	}

	// Further validation will happen lazily when config is accessed
	return nil
}

// validateAutoConfDisabled checks for "auto" values when AutoConf is disabled and logs errors
func validateAutoConfDisabled(cfg *Config) error {
	hasAutoValues := false
	var errors []string

	// Check Bootstrap
	for _, peer := range cfg.Bootstrap {
		if peer == AutoPlaceholder {
			hasAutoValues = true
			errors = append(errors, "Bootstrap contains 'auto' but AutoConf.Enabled=false")
			break
		}
	}

	// Check DNS.Resolvers
	if cfg.DNS.Resolvers != nil {
		for _, resolver := range cfg.DNS.Resolvers {
			if resolver == AutoPlaceholder {
				hasAutoValues = true
				errors = append(errors, "DNS.Resolvers contains 'auto' but AutoConf.Enabled=false")
				break
			}
		}
	}

	// Check Routing.DelegatedRouters
	for _, router := range cfg.Routing.DelegatedRouters {
		if router == AutoPlaceholder {
			hasAutoValues = true
			errors = append(errors, "Routing.DelegatedRouters contains 'auto' but AutoConf.Enabled=false")
			break
		}
	}

	// Check Ipns.DelegatedPublishers
	for _, publisher := range cfg.Ipns.DelegatedPublishers {
		if publisher == AutoPlaceholder {
			hasAutoValues = true
			errors = append(errors, "Ipns.DelegatedPublishers contains 'auto' but AutoConf.Enabled=false")
			break
		}
	}

	// Log all errors
	for _, errMsg := range errors {
		autoconfLog.Error(errMsg)
	}

	// If only auto values exist and no static ones, fail to start
	if hasAutoValues {
		if len(cfg.Bootstrap) == 1 && cfg.Bootstrap[0] == AutoPlaceholder {
			autoconfLog.Error("Kubo cannot start with only 'auto' Bootstrap values when AutoConf.Enabled=false")
			return fmt.Errorf("no usable bootstrap peers: AutoConf is disabled (AutoConf.Enabled=false) but 'auto' placeholder is used in Bootstrap config. Either set AutoConf.Enabled=true to enable automatic configuration, or replace 'auto' with specific Bootstrap peer addresses")
		}
	}

	return nil
}
