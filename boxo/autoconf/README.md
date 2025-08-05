# AutoConf Library

This package provides a client library for fetching and caching IPFS autoconf.json files. It's designed to be generic and self-contained for easy extraction to the boxo library later.

## Features

- **HTTP Client**: Configurable HTTP client with timeout, user-agent, and TLS options
- **Caching**: Intelligent caching with ETag/Last-Modified support and version rotation
- **Version Management**: Automatic cleanup of old cached versions
- **Path Safety**: Sanitized cache directory structure based on URL hostname
- **Error Handling**: Graceful fallback to cached versions when remote fetch fails

## Usage

### Turn-Key Solutions (Recommended)

For most applications, use the "Must" methods that provide graceful fallbacks and never fail:

```go
import "github.com/ipfs/kubo/boxo/autoconf"

const autoconfURL = "https://config.ipfs-mainnet.org/autoconf.json"

// Create a client with default options
client, err := autoconf.NewClient()
if err != nil {
    // handle
}

// Cache-first approach: use cached config, no network requests
config := client.MustGetConfigCached(autoconfURL, autoconf.GetMainnetFallbackConfig)

// Use the config data
nativelySupported := []string{"AminoDHT"}  // Systems your node runs locally (natively)
bootstrapPeers := config.GetBootstrapPeers(nativelySupported...) // Bootstrap every native system
delegatedEndpoints := config.GetDelegatedEndpoints(nativelySupported...) // Excludes native systems
dnsResolvers := config.GetDNSResolvers()

fmt.Printf("Bootstrap peers: %v\n", bootstrapPeers)
fmt.Printf("Delegated endpoints: %v\n", delegatedEndpoints)
fmt.Printf("DNS resolvers: %v\n", dnsResolvers)
```

```go
// With refresh: attempt network update, fall back to cache or defaults
config := client.MustGetConfigWithRefresh(ctx, autoconfURL,
    autoconf.DefaultRefreshInterval, autoconf.GetMainnetFallbackConfig)

// Access config the same way - guaranteed to never be nil
nativelySupported := []string{"AminoDHT"}  // Systems your node runs locally (natively)
bootstrapPeers := config.GetBootstrapPeers(nativelySupported...) // Bootstrap every native system
fmt.Printf("Bootstrap peers: %v\n", bootstrapPeers)
```

### Fine-Grained Control

For applications that need explicit error handling and network control:

```go
// Create client with custom options
client, err := autoconf.NewClient(
    autoconf.WithCacheDir("/path/to/cache"),
    autoconf.WithUserAgent("my-app/1.0"),
    autoconf.WithCacheSize(autoconf.DefaultCacheSize),
    autoconf.WithTimeout(autoconf.DefaultTimeout),
)
if err != nil {
    // handle
}

// GetLatest returns errors for explicit handling
response, err := client.GetLatest(ctx, autoconfURL, autoconf.DefaultRefreshInterval)
if err != nil {
    // handle
    return err
}

// Access the config and metadata
config := response.Config
fmt.Printf("Config: %+v\n", config)
fmt.Printf("FetchTime: %v\n", response.FetchTime)
fmt.Printf("Version: %s\n", response.Version)
fmt.Printf("FromCache: %v\n", response.FromCache)
fmt.Printf("CacheAge: %v\n", response.CacheAge)

// Use all available getters
nativelySupported := []string{"AminoDHT"}  // Systems your node runs locally (natively)
bootstrapPeers := config.GetBootstrapPeers(nativelySupported...) // Bootstrap every native system
delegatedEndpoints := config.GetDelegatedEndpoints(nativelySupported...) // Excludes native systems
allEndpoints := config.GetDelegatedEndpoints() // All endpoints
dnsResolvers := config.GetDNSResolvers()

fmt.Printf("Bootstrap peers: %v\n", bootstrapPeers)
fmt.Printf("Delegated endpoints: %v\n", delegatedEndpoints)
fmt.Printf("All endpoints: %v\n", allEndpoints)
fmt.Printf("DNS resolvers: %v\n", dnsResolvers)
```

### Integration with Applications

This library is designed to be integrated into applications that need autoconf functionality. The client is generic and does not depend on any specific application framework.

```go
func main() {
    client, err := autoconf.NewClient(
        autoconf.WithCacheDir("/app/data/autoconf"),
        autoconf.WithUserAgent("myapp/1.2.3"),
        autoconf.WithTimeout(autoconf.DefaultTimeout),
    )
    if err != nil {
        // handle
    }

    // Use turn-key solution with refresh - never fails, always returns usable config
    config := client.MustGetConfigWithRefresh(context.Background(),
        autoconfURL,
        autoconf.DefaultRefreshInterval,
        autoconf.GetMainnetFallbackConfig)

    // Access all config types
    nativelySupported := []string{"AminoDHT"}  // Systems your node runs locally (natively)
    bootstrapPeers := config.GetBootstrapPeers(nativelySupported...) // Bootstrap every native system
    delegatedEndpoints := config.GetDelegatedEndpoints(nativelySupported...)
    dnsResolvers := config.GetDNSResolvers()

    fmt.Printf("Bootstrap peers: %v\n", bootstrapPeers)
    fmt.Printf("Delegated endpoints: %v\n", delegatedEndpoints)
    fmt.Printf("DNS resolvers: %v\n", dnsResolvers)
}
```

### Background Updates

For applications that need periodic config updates, use the background updater:

```go
func main() {
    client, err := autoconf.NewClient(
        autoconf.WithCacheDir("/app/data/autoconf"),
        autoconf.WithUserAgent("myapp/1.2.3"),
    )
    if err != nil {
        // handle
    }

    // Create background updater with custom callbacks
    updater, err := autoconf.NewBackgroundUpdater(client, autoconfURL,
        autoconf.WithUpdateInterval(autoconf.DefaultRefreshInterval), // Default: 24h
        autoconf.WithOnVersionChange(func(oldVersion, newVersion int64, configURL string) {
            fmt.Printf("New config version %d available (was %d) - consider restarting\n", newVersion, oldVersion)
        }),
        autoconf.WithOnUpdateSuccess(func(resp *autoconf.Response) {
            fmt.Printf("Updated config cache at %s\n", resp.FetchTime.Format(time.RFC3339))
        }),
        autoconf.WithOnUpdateError(func(err error) {
            fmt.Printf("Update failed: %v\n", err)
        }),
    )
    if err != nil {
        // handle
    }

    ctx := context.Background()
    if err := updater.Start(ctx); err != nil {
        // handle
    }
    defer updater.Stop()

    // Your application logic here...
    select {}
}
```

## Cache Structure

The cache is organized by hostname and version for efficient management:

```
$CACHE_DIR/
  autoconf/
    example.com/                    # Hostname-based directory
      autoconf-2025071801.json      # Versioned config files
      autoconf-2025071802.json      # Multiple versions kept for reliability
      .etag                         # HTTP ETag for conditional requests
      .last-modified                # HTTP Last-Modified for cache validation
      .last-refresh                 # Timestamp of last refresh attempt
    config.ipfs-mainnet.org/
      autoconf-2025072301.json
      .etag
      .last-modified
      .last-refresh
```

- **Version-based files**: Each AutoConfVersion gets its own JSON file
- **Metadata files**: Store HTTP headers for efficient conditional requests
- **Automatic cleanup**: Old versions are automatically removed (default: keep 3 versions)
- **Hostname separation**: Multiple URLs cached independently

## Configuration Options

### Client Options
- `WithCacheDir(dir)`: Set custom cache directory
- `WithHTTPClient(client)`: Use custom HTTP client
- `WithCacheSize(n)`: Set maximum cached versions (default: 3)
- `WithUserAgent(ua)`: Set HTTP user-agent
- `WithTimeout(duration)`: Set HTTP timeout (default: 5s)
- `WithTLSInsecureSkipVerify(bool)`: Skip TLS verification (for testing)

### Background Updater Options
- `WithUpdateInterval(duration)`: Set update check interval (default: 24h)
- `WithOnVersionChange(callback)`: Called when new config version is detected
- `WithOnUpdateSuccess(callback)`: Called on successful update for metadata persistence
- `WithOnUpdateError(callback)`: Called when update fails

## Error Handling

The client implements graceful fallback:
1. Try to fetch from remote URL
2. If remote fails, fall back to latest cached version
3. If no cache exists, fall back to provided defaults (when using `MustGetConfigCached` or `MustGetConfigWithRefresh`)

Note: `GetLatest()` returns errors, while `MustGetConfigCached()` and `MustGetConfigWithRefresh()` never fail and always return usable configuration.

## API Overview

### Turn-Key Methods (Recommended)
- **`MustGetConfigCached(url, fallback)`**: Cache-first, never fails, no network requests
- **`MustGetConfigWithRefresh(ctx, url, refreshInterval, fallback)`**: Attempts refresh, falls back gracefully, never fails

### Fine-Grained Methods
- **`GetLatest(ctx, url, refreshInterval)`**: Explicit error handling, returns Response with metadata
- **`NewBackgroundUpdater(client, url, options...)`**: Periodic refresh daemon with customizable callbacks

### Client Creation
- **`NewClient(options...)`**: Create configurable HTTP client with custom cache dir, timeouts, user-agent

### Config Data Access
- **`GetBootstrapPeers(systems...)`**: Extract bootstrap peers for specified systems
- **`GetDelegatedEndpoints(ignoredSystems...)`**: Get HTTP endpoints (optionally excluding native systems)
- **`GetDNSResolvers()`**: Get all DNS-over-HTTPS resolver mappings

## Testing

Run tests with:
```bash
go test .
```

## Future Extraction

This package is designed to be extracted to `github.com/ipfs/boxo` with no changes needed. It has no application-specific dependencies.