# AutoConf Library

This package provides a client library for fetching and caching IPFS autoconf.json files. It's designed to be generic and self-contained for easy extraction to the boxo library later.

## Features

- **HTTP Client**: Configurable HTTP client with timeout, user-agent, and TLS options
- **Caching**: Intelligent caching with ETag/Last-Modified support and version rotation
- **Version Management**: Automatic cleanup of old cached versions
- **Path Safety**: Sanitized cache directory structure based on URL hostname
- **Error Handling**: Graceful fallback to cached versions when remote fetch fails

## Usage

### Basic Usage

```go
import "github.com/ipfs/kubo/boxo/autoconf"

// Create a client with default options
client, err := autoconf.NewClient()
if err != nil {
    return err
}

// Fetch the latest config
config, err := client.GetLatest(ctx, "https://example.com/autoconf.json", autoconf.DefaultRefreshInterval)
if err != nil {
    return err
}

// Use the config
bootstrapPeers := config.GetBootstrapPeers([]string{"AminoDHT"})
fmt.Printf("Bootstrap peers: %v\n", bootstrapPeers)
fmt.Printf("DNS resolvers: %v\n", config.DNSResolvers)
```

### With Custom Options

```go
client, err := autoconf.NewClient(
    autoconf.WithCacheDir("/path/to/cache"),
    autoconf.WithUserAgent("my-app/1.0"),
    autoconf.WithCacheSize(5),
    autoconf.WithTimeout(10*time.Second),
)
```

### Integration with Applications

This library is designed to be integrated into applications that need autoconf functionality. The client is generic and does not depend on any specific application framework.

```go
func main() {
    client, err := autoconf.NewClient(
        autoconf.WithCacheDir("/app/data/autoconf"),
        autoconf.WithUserAgent("myapp/1.2.3"),
        autoconf.WithTimeout(10*time.Second),
    )
    if err != nil {
        panic(err)
    }

    config, err := client.GetLatest(context.Background(), "https://example.com/autoconf.json", autoconf.DefaultRefreshInterval)
    if err != nil {
        panic(err)
    }

    bootstrapPeers := config.GetBootstrapPeers([]string{"AminoDHT"})
    fmt.Printf("Bootstrap peers: %v\n", bootstrapPeers)
    fmt.Printf("DNS resolvers: %v\n", config.DNSResolvers)
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
        panic(err)
    }

    // Create background updater with custom callbacks
    updater, err := autoconf.NewBackgroundUpdater(client, "https://example.com/autoconf.json",
        autoconf.WithUpdateInterval(6*time.Hour), // Check every 6 hours
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
        panic(err)
    }

    ctx := context.Background()
    if err := updater.Start(ctx); err != nil {
        panic(err)
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

### High-Level Functions

- **`NewClient(options...)`**: Create configurable HTTP client with custom cache dir, timeouts, user-agent
- **`GetLatest(ctx, url, refreshInterval)`**: Fetch latest config with cache fallback, returns errors
- **`MustGetConfigCached(client, url, fallback)`**: Never-fail cache-first access, returns fallback if no cache
- **`MustGetConfigWithRefresh(client, url, fallback, refreshInterval)`**: Never-fail with refresh attempt, guaranteed to return usable config
- **`NewBackgroundUpdater(client, url, options...)`**: Periodic refresh daemon with customizable callbacks

## Testing

Run tests with:
```bash
go test .
```

## Future Extraction

This package is designed to be extracted to `github.com/ipfs/boxo` with no changes needed. It has no application-specific dependencies.