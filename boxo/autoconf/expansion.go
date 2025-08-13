package autoconf

import (
	"math/rand"
	"strings"
)

// expandAutoConfSlice is a generic helper for expanding "auto" placeholders in string slices
// It handles the common pattern of: iterate through slice, expand "auto" once, keep custom values
func expandAutoConfSlice(sourceSlice []string, autoConfData []string) []string {
	var resolved []string
	autoExpanded := false

	for _, item := range sourceSlice {
		if item == AutoPlaceholder {
			// Replace with autoconf data (only once)
			if autoConfData != nil && !autoExpanded {
				resolved = append(resolved, autoConfData...)
				autoExpanded = true
			}
			// If autoConfData is nil or already expanded, skip redundant "auto" entries silently
		} else {
			// Keep custom item
			resolved = append(resolved, item)
		}
	}

	return resolved
}

// selectRandom picks a random item from a list for load balancing
func selectRandom(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[rand.Intn(len(items))]
}

// ExpandDNSResolvers expands DNS resolvers with "auto" values replaced by autoconf values.
//
// Parameters:
//   - configResolvers: Map of domain patterns to DNS resolver URLs (e.g., {"eth.": "auto", ".": "https://dns.google/dns-query"});
//     values can be "auto" to use autoconf-provided resolvers
//   - autoConf: The autoconf.json configuration containing DNS resolver mappings; nil disables expansion
//
// Returns map of domains to DNS-over-HTTPS resolver URLs with "auto" expanded.
// When wildcard domain "." is set to "auto", all autoconf resolvers are added.
func ExpandDNSResolvers(configResolvers map[string]string, autoConf *Config) map[string]string {
	if configResolvers == nil {
		// If no resolvers configured, use autoconf defaults if available
		if autoConf != nil && autoConf.DNSResolvers != nil {
			resolved := make(map[string]string)
			for domain, resolvers := range autoConf.DNSResolvers {
				if len(resolvers) > 0 {
					resolved[domain] = selectRandom(resolvers)
				}
			}
			return resolved
		}
		return nil
	}

	resolved := make(map[string]string)
	autoExpanded := 0

	// Check if wildcard "." domain is set to "auto"
	hasWildcardAuto := false
	if resolver, exists := configResolvers["."]; exists && resolver == AutoPlaceholder {
		hasWildcardAuto = true
	}

	// Process each configured resolver
	for domain, resolver := range configResolvers {
		if resolver == AutoPlaceholder {
			// Try to resolve from autoconf
			if autoConf != nil && autoConf.DNSResolvers != nil {
				if resolvers, exists := autoConf.DNSResolvers[domain]; exists && len(resolvers) > 0 {
					resolved[domain] = selectRandom(resolvers)
					autoExpanded++
				}
			}
			// If autoConf is disabled or domain not found, skip this "auto" resolver
		} else {
			// Keep custom resolver as-is
			resolved[domain] = resolver
		}
	}

	// If wildcard "." was set to "auto", add all autoconf resolvers that aren't already configured
	if hasWildcardAuto && autoConf != nil && autoConf.DNSResolvers != nil {
		for domain, resolvers := range autoConf.DNSResolvers {
			if _, exists := resolved[domain]; !exists && len(resolvers) > 0 {
				resolved[domain] = selectRandom(resolvers)
				autoExpanded++
			}
		}
	}

	// Log expansion statistics
	if autoExpanded > 0 {
		log.Debugf("expanded %d 'auto' DNS.Resolvers from autoconf", autoExpanded)
	}

	return resolved
}

// ExpandBootstrapPeers expands bootstrap peers with "auto" values replaced by autoconf values.
//
// Parameters:
//   - configPeers: The user's configured bootstrap peers list, may contain "auto" placeholders to be expanded
//   - autoConf: The autoconf.json configuration fetched from network; nil disables expansion
//   - nativeSystems: List of systems to run natively (e.g., ["AminoDHT"]);
//     used to select appropriate bootstrap peers for those native systems
//
// Returns expanded list of bootstrap peer multiaddrs with "auto" replaced by peers from native systems.
func ExpandBootstrapPeers(configPeers []string, autoConf *Config, nativeSystems []string) []string {
	var autoConfData []string

	if autoConf != nil {
		autoConfData = autoConf.GetBootstrapPeers(nativeSystems...)
		log.Debugf("ExpandBootstrapPeers: processing with %d native systems", len(nativeSystems))
	} else {
		log.Debugf("ExpandBootstrapPeers: autoConf disabled, using original config")
	}

	result := expandAutoConfSlice(configPeers, autoConfData)
	log.Debugf("ExpandBootstrapPeers: final result contains %d peers", len(result))
	return result
}

// ExpandDelegatedEndpoints expands delegated endpoints with "auto" values replaced by autoconf values.
//
// Parameters:
//   - configEndpoints: The user's configured endpoints list, may contain "auto" placeholders to be expanded
//   - autoConf: The autoconf.json configuration fetched from network; nil disables expansion
//   - nativeSystems: List of systems to run natively (e.g., ["AminoDHT"]);
//     these systems are excluded from delegated endpoints to avoid duplication
//   - supportedPaths: Optional filter for which routing paths to include (e.g., "/routing/v1/providers");
//     if empty, all paths are included
//
// Returns expanded list of endpoint URLs with "auto" replaced by filtered autoconf endpoints.
// Endpoints are filtered to exclude those overlapping with native systems and unsupported paths.
func ExpandDelegatedEndpoints(configEndpoints []string, autoConf *Config, nativeSystems []string, supportedPaths ...string) []string {
	// Filter autoconf to only include supported paths
	if autoConf != nil && len(supportedPaths) > 0 {
		autoConf = autoConf.WithSupportedPathsOnly(supportedPaths...)
	}

	var endpoints map[string]EndpointConfig
	if autoConf != nil {
		endpoints = autoConf.GetDelegatedEndpoints(nativeSystems...)
		log.Debugf("ExpandDelegatedEndpoints: processing with %d native systems", len(nativeSystems))
	} else {
		log.Debugf("ExpandDelegatedEndpoints: autoConf disabled, using original config")
	}

	if endpoints == nil {
		result := expandAutoConfSlice(configEndpoints, nil)
		return result
	}

	var routers []string
	for baseURL, config := range endpoints {
		// Combine both Read and Write paths (already filtered by WithSupportedPathsOnly)
		allPaths := append(config.Read, config.Write...)

		// Build URLs for all paths
		urls := buildEndpointURLs(baseURL, allPaths)
		routers = append(routers, urls...)
	}

	resolved := expandAutoConfSlice(configEndpoints, routers)

	// Filter out URLs with unsupported routing paths
	resolved = filterValidRoutingURLs(resolved)

	// Final safety check to guarantee no trailing slashes
	for i, url := range resolved {
		resolved[i] = strings.TrimRight(url, "/")
	}

	log.Debugf("ExpandDelegatedEndpoints: final result contains %d endpoints", len(resolved))
	return resolved
}
