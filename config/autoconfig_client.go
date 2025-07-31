package config

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ipfs/kubo/boxo/autoconfig"

	logging "github.com/ipfs/go-log/v2"
)

// NewAutoConfigClient creates an autoconfig client with standard defaults
func NewAutoConfigClient(repoPath, userAgent string) (*autoconfig.Client, error) {
	cacheDir := filepath.Join(repoPath, "autoconfig")
	return autoconfig.NewClient(
		autoconfig.WithCacheDir(cacheDir),
		autoconfig.WithUserAgent(userAgent),
		autoconfig.WithCacheSize(DefaultAutoconfigCacheSize),
		autoconfig.WithTimeout(DefaultAutoconfigTimeout),
	)
}

// NewAutoConfigClientWithConfig creates an autoconfig client with config-specific settings
func NewAutoConfigClientWithConfig(repoPath string, cfg interface{}, userAgent string) (*autoconfig.Client, error) {
	cacheDir := filepath.Join(repoPath, "autoconfig")
	options := []autoconfig.Option{
		autoconfig.WithCacheDir(cacheDir),
		autoconfig.WithUserAgent(userAgent),
		autoconfig.WithCacheSize(DefaultAutoconfigCacheSize),
		autoconfig.WithTimeout(DefaultAutoconfigTimeout),
	}

	// Add TLS skip verify if config provides it
	type tlsConfig interface {
		GetTLSInsecureSkipVerify() bool
	}
	if config, ok := cfg.(tlsConfig); ok && config.GetTLSInsecureSkipVerify() {
		options = append(options, autoconfig.WithTLSInsecureSkipVerify(true))
	}

	return autoconfig.NewClient(options...)
}

// GetAutoConfig is a convenience function to get the latest config with a default client
// Uses the default check interval - for user-configured intervals, use GetLatest directly
func GetAutoConfig(ctx context.Context, configURL, repoPath, userAgent string) (*autoconfig.Config, error) {
	client, err := NewAutoConfigClient(repoPath, userAgent)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetLatest(ctx, configURL, autoconfig.DefaultRefreshInterval)
	if err != nil {
		return nil, err
	}
	return resp.Config, nil
}

// GetAutoConfigWithMetadata is a convenience function to get the latest config with metadata using a default client
// Uses a default check interval of 24 hours
func GetAutoConfigWithMetadata(ctx context.Context, configURL, repoPath, userAgent string) (*autoconfig.Response, error) {
	client, err := NewAutoConfigClient(repoPath, userAgent)
	if err != nil {
		return nil, err
	}
	return client.GetLatest(ctx, configURL, autoconfig.DefaultRefreshInterval)
}

// GetAutoConfigFromCacheOnly is a convenience function to get cached autoconfig without trying to fetch from remote
func GetAutoConfigFromCacheOnly(repoPath string) (*autoconfig.Config, error) {
	// Since this is cache-only, no network requests are made, so user agent is not needed
	client, err := autoconfig.NewClient()
	if err != nil {
		return nil, err
	}
	cacheDir := filepath.Join(repoPath, "autoconfig")
	return client.GetCachedConfig(cacheDir)
}

// ValidateAutoConfigAtStartup validates that autoconfig setup is correct at daemon startup
func ValidateAutoConfigAtStartup(cfg *Config) error {
	if !cfg.AutoConfig.Enabled.WithDefault(DefaultAutoConfigEnabled) {
		// AutoConfig is disabled, check for "auto" values and warn
		return validateAutoConfigDisabled(cfg)
	}

	// AutoConfig is enabled - validate URL is provided
	if cfg.AutoConfig.URL == "" {
		return fmt.Errorf("AutoConfig is enabled but AutoConfig.URL is empty - please provide a URL")
	}

	// Further validation will happen lazily when config is accessed
	return nil
}

// ValidateAutoConfigWithRepo validates that autoconfig setup is correct at daemon startup with repo access
func ValidateAutoConfigWithRepo(cfg *Config, swarmKeyExists bool) error {
	if !cfg.AutoConfig.Enabled.WithDefault(DefaultAutoConfigEnabled) {
		// AutoConfig is disabled, check for "auto" values and warn
		return validateAutoConfigDisabled(cfg)
	}

	// AutoConfig is enabled - validate URL is provided
	if cfg.AutoConfig.URL == "" {
		return fmt.Errorf("AutoConfig is enabled but AutoConfig.URL is empty - please provide a URL")
	}

	// Check for private network with default mainnet URL
	if swarmKeyExists && cfg.AutoConfig.URL == DefaultAutoConfigURL {
		return fmt.Errorf("AutoConfig cannot use the default mainnet URL (%s) on a private network (swarm.key or LIBP2P_FORCE_PNET detected). Either disable AutoConfig by setting AutoConfig.Enabled=false, or configure AutoConfig.URL to point to a configuration service specific to your private swarm", DefaultAutoConfigURL)
	}

	// Further validation will happen lazily when config is accessed
	return nil
}

var autoconfigLog = logging.Logger("autoconfig")

// validateAutoConfigDisabled checks for "auto" values when AutoConfig is disabled and logs errors
func validateAutoConfigDisabled(cfg *Config) error {
	hasAutoValues := false
	var errors []string

	// Check Bootstrap
	for _, peer := range cfg.Bootstrap {
		if peer == AutoPlaceholder {
			hasAutoValues = true
			errors = append(errors, "Bootstrap contains 'auto' but AutoConfig.Enabled=false")
			break
		}
	}

	// Check DNS.Resolvers
	if cfg.DNS.Resolvers != nil {
		for _, resolver := range cfg.DNS.Resolvers {
			if resolver == AutoPlaceholder {
				hasAutoValues = true
				errors = append(errors, "DNS.Resolvers contains 'auto' but AutoConfig.Enabled=false")
				break
			}
		}
	}

	// Check Routing.DelegatedRouters
	for _, router := range cfg.Routing.DelegatedRouters {
		if router == AutoPlaceholder {
			hasAutoValues = true
			errors = append(errors, "Routing.DelegatedRouters contains 'auto' but AutoConfig.Enabled=false")
			break
		}
	}

	// Check Ipns.DelegatedPublishers
	for _, publisher := range cfg.Ipns.DelegatedPublishers {
		if publisher == AutoPlaceholder {
			hasAutoValues = true
			errors = append(errors, "Ipns.DelegatedPublishers contains 'auto' but AutoConfig.Enabled=false")
			break
		}
	}

	// Log all errors
	for _, errMsg := range errors {
		autoconfigLog.Error(errMsg)
	}

	// If only auto values exist and no static ones, fail to start
	if hasAutoValues {
		if len(cfg.Bootstrap) == 1 && cfg.Bootstrap[0] == AutoPlaceholder {
			autoconfigLog.Error("Kubo cannot start with only 'auto' Bootstrap values when AutoConfig.Enabled=false")
			return fmt.Errorf("no usable bootstrap peers: AutoConfig is disabled (AutoConfig.Enabled=false) but 'auto' placeholder is used in Bootstrap config. Either set AutoConfig.Enabled=true to enable automatic configuration, or replace 'auto' with specific Bootstrap peer addresses")
		}
	}

	return nil
}
