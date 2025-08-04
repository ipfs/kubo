package config

import (
	"fmt"
	"path/filepath"

	"github.com/ipfs/kubo/boxo/autoconf"

	logging "github.com/ipfs/go-log/v2"
)

// NewAutoConfClient creates an autoconf client with standard defaults
func NewAutoConfClient(repoPath, userAgent string) (*autoconf.Client, error) {
	cacheDir := filepath.Join(repoPath, "autoconf")
	return autoconf.NewClient(
		autoconf.WithCacheDir(cacheDir),
		autoconf.WithUserAgent(userAgent),
		autoconf.WithCacheSize(DefaultAutoConfCacheSize),
		autoconf.WithTimeout(DefaultAutoConfTimeout),
	)
}

// ValidateAutoConfWithRepo validates that autoconf setup is correct at daemon startup with repo access
func ValidateAutoConfWithRepo(cfg *Config, swarmKeyExists bool) error {
	if !cfg.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled) {
		// AutoConf is disabled, check for "auto" values and warn
		return validateAutoConfDisabled(cfg)
	}

	// AutoConf is enabled - validate URL is provided
	if cfg.AutoConf.URL == "" {
		return fmt.Errorf("AutoConf is enabled but AutoConf.URL is empty - please provide a URL")
	}

	// Check for private network with default mainnet URL
	if swarmKeyExists && cfg.AutoConf.URL == DefaultAutoConfURL {
		return fmt.Errorf("AutoConf cannot use the default mainnet URL (%s) on a private network (swarm.key or LIBP2P_FORCE_PNET detected). Either disable AutoConf by setting AutoConf.Enabled=false, or configure AutoConf.URL to point to a configuration service specific to your private swarm", DefaultAutoConfURL)
	}

	// Further validation will happen lazily when config is accessed
	return nil
}

var autoconfLog = logging.Logger("autoconf")

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
