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

// Create a client with default options (uses mainnet URL and fallback)
client, err := autoconf.NewClient()
if err != nil {
    // handle
}

// Cache-first approach: use cached config, no network requests
config := client.GetCached()

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
config := client.GetCachedOrRefresh(ctx)

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
    autoconf.WithURL("https://conf.ipfs-mainnet.org/autoconf.json"),
    autoconf.WithRefreshInterval(24*time.Hour),
    autoconf.WithCacheDir("/path/to/cache"),
    autoconf.WithUserAgent("my-app/1.0"),
    autoconf.WithCacheSize(autoconf.DefaultCacheSize),
    autoconf.WithTimeout(autoconf.DefaultTimeout),
    autoconf.WithFallback(autoconf.GetMainnetFallbackConfig), // Explicit fallback
)
if err != nil {
    // handle
}

// GetLatest returns errors for explicit handling
response, err := client.GetLatest(ctx)
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

For long-running applications that need periodic config updates:

#### Pattern 1: Context-based lifecycle (recommended for service integration)

```go
func runService(ctx context.Context) error {
    client, err := autoconf.NewClient(
        autoconf.WithCacheDir("/app/data/autoconf"),
        autoconf.WithUserAgent("myapp/1.2.3"),
        autoconf.WithRefreshInterval(12*time.Hour),
    )
    if err != nil {
        return err
    }

    // Start primes cache and starts background updater
    // Updater will stop automatically when ctx is cancelled
    config, err := client.Start(ctx)
    if err != nil {
        return err
    }
    
    // Use the config immediately
    bootstrapPeers := config.GetBootstrapPeers("AminoDHT")
    
    // Your service logic here...
    // When ctx is cancelled, the updater stops automatically
    return nil
}
```

#### Pattern 2: Explicit Stop() method (recommended for manual control)

```go
func main() {
    client, err := autoconf.NewClient(
        autoconf.WithCacheDir("/app/data/autoconf"),
        autoconf.WithUserAgent("myapp/1.2.3"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Start with background context since we'll use Stop()
    config, err := client.Start(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    
    // Ensure clean shutdown
    defer client.Stop()
    
    // Set up signal handling
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    
    // Your application logic here...
    <-sigCh
    
    // Stop() will be called by defer, ensuring graceful shutdown
}
```

Both patterns are idiomatic in Go. Use context cancellation when integrating with existing service frameworks, and use Stop() when you need explicit control over the updater lifecycle.

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
    conf.ipfs-mainnet.org/
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
3. If no cache exists, fall back to configured fallback function

Note: `GetLatest()` returns errors for explicit handling, while `GetCached()` and `GetCachedOrRefresh()` never fail and always return usable configuration (using the fallback if necessary).

## API Overview

### Client Methods
- **`NewClient(options...)`**: Create configurable client with cache dir, timeouts, user-agent
- **`GetCached()`**: Returns cached config or fallback, no network requests
- **`GetCachedOrRefresh(ctx)`**: Returns latest config if stale, falls back to cache/default
- **`GetLatest(ctx)`**: Explicitly fetches from network, returns error for handling
- **`Start(ctx)`**: Primes cache and starts background updater, returns initial config
- **`Stop()`**: Gracefully stops the background updater (alternative to context cancellation)

### Config Data Access Methods
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