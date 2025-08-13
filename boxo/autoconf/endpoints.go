package autoconf

// hasSystemOverlap checks if any of the endpoint's systems overlap with native systems
func hasSystemOverlap(endpointSystems []string, nativeSystems []string) bool {
	for _, system := range endpointSystems {
		for _, native := range nativeSystems {
			if system == native {
				return true
			}
		}
	}
	return false
}

// DelegatedEndpointsForRead returns delegated endpoints configured for read operations,
// filtering out those that overlap with native systems
func (c *Config) DelegatedEndpointsForRead(ignoredNativeSystems ...string) map[string]EndpointConfig {
	if c.DelegatedEndpoints == nil {
		return nil
	}

	filtered := make(map[string]EndpointConfig)

	for url, conf := range c.DelegatedEndpoints {
		// Skip if overlaps with native systems or has no read operations
		if hasSystemOverlap(conf.Systems, ignoredNativeSystems) || len(conf.Read) == 0 {
			continue
		}

		// Create a copy with only read operations
		filtered[url] = EndpointConfig{
			Systems: conf.Systems,
			Read:    conf.Read,
			Write:   []string{}, // Exclude write operations
		}
	}

	log.Debugf("DelegatedEndpointsForRead: returning %d endpoints from %d total", len(filtered), len(c.DelegatedEndpoints))
	return filtered
}

// DelegatedEndpointsForWrite returns delegated endpoints configured for write operations,
// filtering out those that overlap with native systems
func (c *Config) DelegatedEndpointsForWrite(ignoredNativeSystems ...string) map[string]EndpointConfig {
	if c.DelegatedEndpoints == nil {
		return nil
	}

	filtered := make(map[string]EndpointConfig)

	for url, conf := range c.DelegatedEndpoints {
		// Skip if overlaps with native systems or has no write operations
		if hasSystemOverlap(conf.Systems, ignoredNativeSystems) || len(conf.Write) == 0 {
			continue
		}

		// Create a copy with only write operations
		filtered[url] = EndpointConfig{
			Systems: conf.Systems,
			Read:    []string{}, // Exclude read operations
			Write:   conf.Write,
		}
	}

	log.Debugf("DelegatedEndpointsForWrite: returning %d endpoints from %d total", len(filtered), len(c.DelegatedEndpoints))
	return filtered
}

// WithSupportedPathsOnly returns a new Config with DelegatedEndpoints filtered to only include
// endpoints that have at least one path matching the supported paths list.
// This allows applications to filter endpoints based on their specific needs.
func (c *Config) WithSupportedPathsOnly(supportedPaths ...string) *Config {
	if c == nil || len(supportedPaths) == 0 {
		return c
	}

	// Create a copy of the config
	filtered := &Config{
		AutoConfVersion: c.AutoConfVersion,
		AutoConfSchema:  c.AutoConfSchema,
		AutoConfTTL:     c.AutoConfTTL,
		SystemRegistry:  c.SystemRegistry,
		DNSResolvers:    c.DNSResolvers,
	}

	// Filter DelegatedEndpoints
	if c.DelegatedEndpoints != nil {
		filtered.DelegatedEndpoints = make(map[string]EndpointConfig)

		for url, conf := range c.DelegatedEndpoints {
			// Check if any Read or Write path matches the supported paths
			filteredConf := EndpointConfig{
				Systems: conf.Systems,
				Read:    filterPaths(conf.Read, supportedPaths),
				Write:   filterPaths(conf.Write, supportedPaths),
			}

			// Only include endpoint if it has at least one supported path
			if len(filteredConf.Read) > 0 || len(filteredConf.Write) > 0 {
				filtered.DelegatedEndpoints[url] = filteredConf
			}
		}
	}

	return filtered
}

// filterPaths returns only the paths that are in the supported list
func filterPaths(paths []string, supportedPaths []string) []string {
	var filtered []string
	for _, path := range paths {
		for _, supported := range supportedPaths {
			if path == supported {
				filtered = append(filtered, path)
				break
			}
		}
	}
	return filtered
}
