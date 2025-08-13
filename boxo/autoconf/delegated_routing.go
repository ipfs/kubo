package autoconf

import (
	"fmt"
	"net/url"
	"strings"
)

// Routing path constants (with leading slash for proper URL construction)
const (
	RoutingV1ProvidersPath = "/routing/v1/providers"
	RoutingV1PeersPath     = "/routing/v1/peers"
	RoutingV1IPNSPath      = "/routing/v1/ipns"
)

// EndpointCapabilities represents which routing operations are supported by an endpoint
type EndpointCapabilities struct {
	Providers bool // GET /routing/v1/providers
	Peers     bool // GET /routing/v1/peers
	IPNSGet   bool // GET /routing/v1/ipns (IPNS resolution)
	IPNSPut   bool // PUT /routing/v1/ipns (IPNS publishing)
}

// IsEmpty returns true if no capabilities are enabled
func (ec EndpointCapabilities) IsEmpty() bool {
	return !ec.Providers && !ec.Peers && !ec.IPNSGet && !ec.IPNSPut
}

// Merge combines capabilities from another EndpointCapabilities
func (ec *EndpointCapabilities) Merge(other EndpointCapabilities) {
	ec.Providers = ec.Providers || other.Providers
	ec.Peers = ec.Peers || other.Peers
	ec.IPNSGet = ec.IPNSGet || other.IPNSGet
	ec.IPNSPut = ec.IPNSPut || other.IPNSPut
}

// DelegatedRoutingEndpoint represents a parsed routing endpoint with its capabilities
type DelegatedRoutingEndpoint struct {
	BaseURL      string               // Base URL without path (e.g., "https://example.com")
	Capabilities EndpointCapabilities // What operations this endpoint supports
}

// DetermineKnownCapabilities parses a routing URL and determines its capabilities
// It accepts both base URLs and URLs with specific routing paths:
//   - https://example.com → all operations
//   - https://example.com/routing/v1/providers → only provider lookups
//   - https://example.com/routing/v1/peers → only peer lookups
//   - https://example.com/routing/v1/ipns → only IPNS operations
//
// The supportsRead and supportsWrite parameters indicate whether the endpoint
// should be used for read operations (GET) and/or write operations (PUT).
func DetermineKnownCapabilities(endpoint string, supportsRead, supportsWrite bool) (*DelegatedRoutingEndpoint, error) {
	// Parse and validate the URL
	baseURL, path, err := parseAndValidateRoutingURL(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid routing URL %q: %w", endpoint, err)
	}

	// Determine capabilities based on path
	var caps EndpointCapabilities

	switch path {
	case "", "/": // Base URL - all operations supported
		if supportsRead {
			caps.Providers = true
			caps.Peers = true
			caps.IPNSGet = true
		}
		if supportsWrite {
			caps.IPNSPut = true
		}

	case RoutingV1ProvidersPath:
		if supportsRead {
			caps.Providers = true
		}

	case RoutingV1PeersPath:
		if supportsRead {
			caps.Peers = true
		}

	case RoutingV1IPNSPath:
		if supportsRead {
			caps.IPNSGet = true
		}
		if supportsWrite {
			caps.IPNSPut = true
		}

	default:
		// This shouldn't happen as ParseAndValidateRoutingURL validates paths
		return nil, fmt.Errorf("unsupported routing path %q", path)
	}

	return &DelegatedRoutingEndpoint{
		BaseURL:      baseURL,
		Capabilities: caps,
	}, nil
}

// GroupByKnownCapabilities processes a list of endpoint URLs and groups them by base URL,
// merging capabilities for endpoints that share the same base URL.
// This is useful when multiple path-specific URLs point to the same server.
func GroupByKnownCapabilities(endpoints []string, supportsRead, supportsWrite bool) (map[string]EndpointCapabilities, error) {
	grouped := make(map[string]EndpointCapabilities)

	for _, endpoint := range endpoints {
		// Skip empty endpoints
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" {
			continue
		}

		// Parse endpoint and determine capabilities
		parsed, err := DetermineKnownCapabilities(endpoint, supportsRead, supportsWrite)
		if err != nil {
			// Log and skip invalid endpoints rather than failing completely
			log.Debugf("Skipping invalid endpoint %q: %v", endpoint, err)
			continue
		}

		// Merge capabilities for this base URL
		existing := grouped[parsed.BaseURL]
		existing.Merge(parsed.Capabilities)
		grouped[parsed.BaseURL] = existing
	}

	return grouped, nil
}

// extractUniqueBaseURLs processes a list of routing URLs and returns unique base URLs.
// Base URL is the URL without any path (e.g., "https://example.com" from "https://example.com/routing/v1/providers").
// This is useful when you need to create routing clients that expect base URLs.
func extractUniqueBaseURLs(endpoints []string) []string {
	baseURLSet := make(map[string]bool)
	var result []string

	for _, endpoint := range endpoints {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" {
			continue
		}

		baseURL, _, err := parseAndValidateRoutingURL(endpoint)
		if err != nil {
			log.Debugf("Skipping invalid endpoint %q: %v", endpoint, err)
			continue
		}

		if !baseURLSet[baseURL] {
			baseURLSet[baseURL] = true
			result = append(result, baseURL)
		}
	}

	return result
}

// parseAndValidateRoutingURL extracts base URL and validates routing path in one step
// Returns error if URL is invalid or has unsupported routing path
func parseAndValidateRoutingURL(endpoint string) (baseURL string, path string, err error) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", endpoint, err)
	}

	// Build base URL without path
	baseURL = fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Extract path (keep leading slash for comparison)
	path = strings.TrimSuffix(parsedURL.Path, "/")

	// Validate routing path
	switch path {
	case "", "/": // No path or root - base URL
	case RoutingV1ProvidersPath: // Provider lookups
	case RoutingV1PeersPath: // Peer lookups
	case RoutingV1IPNSPath: // IPNS resolution/publishing
		// Valid paths - continue
	default:
		return "", "", fmt.Errorf("unsupported routing path %q", path)
	}

	return baseURL, path, nil
}

// filterValidRoutingURLs filters out URLs with unsupported routing paths
func filterValidRoutingURLs(urls []string) []string {
	var filtered []string
	for _, urlStr := range urls {
		if _, _, err := parseAndValidateRoutingURL(urlStr); err == nil {
			filtered = append(filtered, urlStr)
		} else {
			log.Debugf("Skipping invalid routing URL %q: %v", urlStr, err)
		}
	}
	return filtered
}

// buildEndpointURL constructs a URL from baseURL and path, ensuring no trailing slash
func buildEndpointURL(baseURL, path string) string {
	// Always trim trailing slash from baseURL
	cleanBase := strings.TrimRight(baseURL, "/")

	// Ensure path starts with / if not empty
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Construct and ensure no trailing slash
	fullURL := cleanBase + path
	return strings.TrimRight(fullURL, "/")
}

// buildEndpointURLs creates URLs from base URL and paths, ensuring no trailing slashes
func buildEndpointURLs(baseURL string, paths []string) []string {
	var urls []string
	for _, path := range paths {
		url := buildEndpointURL(baseURL, path)
		urls = append(urls, url)
	}
	return urls
}

// containsPath checks if the given paths contain the target path
func containsPath(paths []string, targetPath string) bool {
	for _, path := range paths {
		if path == targetPath {
			return true
		}
	}
	return false
}
