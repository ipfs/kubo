# AutoConfig Library

This package provides a client library for fetching and caching IPFS autoconfig.json files. It's designed to be generic and self-contained for easy extraction to the boxo library later.

## Features

- **HTTP Client**: Configurable HTTP client with timeout, user-agent, and TLS options
- **Caching**: Intelligent caching with ETag/Last-Modified support and version rotation
- **Version Management**: Automatic cleanup of old cached versions
- **Path Safety**: Sanitized cache directory structure based on URL hostname
- **Error Handling**: Graceful fallback to cached versions when remote fetch fails

## Usage

### Basic Usage

```go
import "github.com/ipfs/kubo/boxo/autoconfig"

// Create a client with default options
client, err := autoconfig.NewClient()
if err != nil {
    return err
}

// Fetch the latest config
config, err := client.GetLatest(ctx, "https://example.com/autoconfig.json", autoconfig.DefaultRefreshInterval)
if err != nil {
    return err
}

// Use the config
fmt.Printf("Bootstrap peers: %v\n", config.Bootstrap)
```

### With Custom Options

```go
client, err := autoconfig.NewClient(
    autoconfig.WithCacheDir("/path/to/cache"),
    autoconfig.WithUserAgent("my-app/1.0"),
    autoconfig.WithCacheSize(5),
    autoconfig.WithTimeout(10*time.Second),
)
```

### Integration with Applications

This library is designed to be integrated into applications that need autoconfig functionality. The client is generic and does not depend on any specific application framework.

```go
func main() {
    client, err := autoconfig.NewClient(
        autoconfig.WithCacheDir("/app/data/autoconfig"),
        autoconfig.WithUserAgent("myapp/1.2.3"),
        autoconfig.WithTimeout(10*time.Second),
    )
    if err != nil {
        panic(err)
    }

    config, err := client.GetLatest(context.Background(), "https://example.com/autoconfig.json", autoconfig.DefaultRefreshInterval)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Bootstrap peers: %v\n", config.Bootstrap)
    fmt.Printf("DNS resolvers: %v\n", config.DNSResolvers)
}
```

### Background Updates

For applications that need periodic config updates, use the background updater:

```go
func main() {
    client, err := autoconfig.NewClient(
        autoconfig.WithCacheDir("/app/data/autoconfig"),
        autoconfig.WithUserAgent("myapp/1.2.3"),
    )
    if err != nil {
        panic(err)
    }

    // Create background updater with custom callbacks
    updater, err := autoconfig.NewBackgroundUpdater(client, "https://example.com/autoconfig.json",
        autoconfig.WithUpdateInterval(6*time.Hour), // Check every 6 hours
        autoconfig.WithOnVersionChange(func(oldVersion, newVersion int64, configURL string) {
            fmt.Printf("New config version %d available (was %d) - consider restarting\n", newVersion, oldVersion)
        }),
        autoconfig.WithOnUpdateSuccess(func(resp *autoconfig.Response) {
            fmt.Printf("Updated config cache at %s\n", resp.FetchTime.Format(time.RFC3339))
        }),
        autoconfig.WithOnUpdateError(func(err error) {
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

The cache is organized by hostname and version:

```
$CACHE_DIR/
  autoconfig/
    example.com/
      2025071801.json
      2025071802.json
      .etag
      .last-modified
```

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
3. If no cache exists, fall back to hardcoded defaults (when using `MustGetConfigWithMainnetFallbacks`)

Note: `GetLatest()` returns errors, while `MustGetConfigWithMainnetFallbacks()` never fails and always returns usable configuration.

## Testing

Run tests with:
```bash
go test .
```

## Future Extraction

This package is designed to be extracted to `github.com/ipfs/boxo` with no changes needed. It has no application-specific dependencies.