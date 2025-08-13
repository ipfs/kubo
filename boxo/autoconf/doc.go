// Package autoconf provides a client for fetching and caching IPFS network configurations.
//
// The client supports intelligent caching with HTTP conditional requests (ETags/Last-Modified),
// automatic fallbacks to cached or default configurations, and background updates.
// Multiple configuration versions are kept locally to ensure reliability during network issues.
//
// # Basic Usage
//
// The simplest way to use autoconf is with the default client:
//
//	client, err := autoconf.NewClient()
//	if err != nil {
//	    // handle error
//	}
//
//	// Get cached config or fallback (no network)
//	config := client.GetCached()
//
//	// Get latest within refresh interval, with fallbacks
//	config := client.GetCachedOrRefresh(ctx)
//
// # Configuration Options
//
// The client can be customized with various options:
//
//	client, err := autoconf.NewClient(
//	    autoconf.WithURL("https://conf.ipfs-mainnet.org/autoconf.json"),
//	    autoconf.WithRefreshInterval(24*time.Hour),
//	    autoconf.WithCacheDir("/path/to/cache"),
//	    autoconf.WithUserAgent("my-app/1.0"),
//	    autoconf.WithFallback(autoconf.GetMainnetFallbackConfig), // Explicit fallback
//	)
//
// # Method Overview
//
// The client provides three main methods for retrieving configuration:
//
//   - GetCached(): Returns cached config or fallback, never makes network requests
//   - GetCachedOrRefresh(ctx): Returns latest config if stale, falls back to cache/default
//   - GetLatest(ctx): Explicitly fetches from network, returns error for handling
//
// # Background Updates
//
// For long-running applications, use Start to prime cache and start background updater:
//
//	config, err := client.Start(ctx)
//	if err != nil {
//	    // handle error
//	}
//	// config is immediately available for use
//
//	// The updater can be stopped in two ways:
//	// 1. Cancel the context passed to Start() (automatic)
//	// 2. Call client.Stop() explicitly (manual)
//	defer client.Stop()  // Optional: explicit shutdown
//
// This primes the cache with the latest config and starts a background service that
// periodically checks for updates and logs when new versions are available.
//
// # Config Expansion
//
// The package also provides utilities to expand "auto" placeholders in configuration:
//
//	// Expand bootstrap peers
//	peers := autoconf.ExpandBootstrapPeers(configPeers, autoConfData, nativeSystems)
//
//	// Expand DNS resolvers
//	resolvers := autoconf.ExpandDNSResolvers(configResolvers, autoConfData)
//
//	// Expand delegated endpoints
//	endpoints := autoconf.ExpandDelegatedEndpoints(configEndpoints, autoConfData, nativeSystems)
package autoconf
