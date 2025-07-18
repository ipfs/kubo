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
import "github.com/ipfs/kubo/internal/autoconfig"

// Create a client with default options
client, err := autoconfig.NewClient()
if err != nil {
    return err
}

// Fetch the latest config
config, err := client.GetLatest(ctx, "https://config.ipfs-mainnet.org/autoconfig.json")
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

### Convenience Functions

```go
// Using the Kubo client with repo path
config, err := autoconfig.Get(ctx, configURL, repoPath)

// Or create a Kubo client directly
client, err := autoconfig.KuboClient(repoPath)
```

## Cache Structure

The cache is organized by hostname and version:

```
$CACHE_DIR/
  autoconfig/
    config.ipfs-mainnet.org/
      2025071801.json
      2025071802.json
      .etag
      .last-modified
```

## Configuration Options

- `WithCacheDir(dir)`: Set custom cache directory
- `WithHTTPClient(client)`: Use custom HTTP client
- `WithCacheSize(n)`: Set maximum cached versions (default: 3)
- `WithUserAgent(ua)`: Set HTTP user-agent
- `WithTimeout(duration)`: Set HTTP timeout (default: 5s)
- `WithTLSInsecureSkipVerify(bool)`: Skip TLS verification (for testing)

## Error Handling

The client implements graceful fallback:
1. Try to fetch from remote URL
2. If remote fails, fall back to latest cached version
3. If no cache exists, return error with details

## Testing

Run tests with:
```bash
go test ./internal/autoconfig/
```

## Future Extraction

This package is designed to be extracted to `github.com/ipfs/boxo` with minimal changes. The only Kubo-specific dependency is the version import for the user-agent.