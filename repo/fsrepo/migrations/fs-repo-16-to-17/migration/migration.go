// package mg16 contains the code to perform 16-17 repository migration in Kubo.
// This handles the following:
// - Migrate default bootstrap peers to "auto"
// - Migrate DNS resolvers to use "auto" for "." eTLD
// - Enable AutoConf system with default settings
// - Increment repo version to 17
package mg16

import (
	"io"
	"slices"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/repo/fsrepo/migrations/common"
)

// DefaultBootstrapAddresses are the hardcoded bootstrap addresses from Kubo 0.36
// for IPFS. they are nodes run by the IPFS team. docs on these later.
// As with all p2p networks, bootstrap is an important security concern.
// This list is used during migration to detect which peers are defaults vs custom.
var DefaultBootstrapAddresses = []string{
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa", // rust-libp2p-server
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	"/dnsaddr/va1.bootstrap.libp2p.io/p2p/12D3KooWKnDdG3iXw9eTFijk3EWSunZcFi54Zka4wmtqtt6rPxc8", // js-libp2p-amino-dht-bootstrapper
	"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",           // mars.i.ipfs.io
	"/ip4/104.131.131.82/udp/4001/quic-v1/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",   // mars.i.ipfs.io
}

// Migration is the main exported migration for 16-to-17
var Migration = &common.BaseMigration{
	FromVersion: "16",
	ToVersion:   "17",
	Description: "Upgrading config to use AutoConf system",
	Convert:     convert,
}

// NewMigration creates a new migration instance (for compatibility)
func NewMigration() common.Migration {
	return Migration
}

// convert converts the config from version 16 to 17
func convert(in io.ReadSeeker, out io.Writer) error {
	confMap, err := common.ReadConfig(in)
	if err != nil {
		return err
	}

	// Enable AutoConf system
	if err := enableAutoConf(confMap); err != nil {
		return err
	}

	// Migrate Bootstrap peers
	if err := migrateBootstrap(confMap); err != nil {
		return err
	}

	// Migrate DNS resolvers
	if err := migrateDNSResolvers(confMap); err != nil {
		return err
	}

	// Migrate DelegatedRouters
	if err := migrateDelegatedRouters(confMap); err != nil {
		return err
	}

	// Migrate DelegatedPublishers
	if err := migrateDelegatedPublishers(confMap); err != nil {
		return err
	}

	// Save new config
	return common.WriteConfig(out, confMap)
}

// enableAutoConf adds AutoConf section to config
func enableAutoConf(confMap map[string]any) error {
	// Add empty AutoConf section if it doesn't exist - all fields will use implicit defaults:
	// - Enabled defaults to true (via DefaultAutoConfEnabled)
	// - URL defaults to mainnet URL (via DefaultAutoConfURL)
	// - RefreshInterval defaults to 24h (via DefaultAutoConfRefreshInterval)
	// - TLSInsecureSkipVerify defaults to false (no WithDefault, but false is zero value)
	common.SetDefault(confMap, "AutoConf", map[string]any{})
	return nil
}

// migrateBootstrap migrates bootstrap peers to use "auto"
func migrateBootstrap(confMap map[string]any) error {
	bootstrap, exists := confMap["Bootstrap"]
	if !exists {
		// No bootstrap section, add "auto"
		confMap["Bootstrap"] = []string{config.AutoPlaceholder}
		return nil
	}

	// Convert to string slice using helper
	bootstrapPeers := common.ConvertInterfaceSlice(common.SafeCastSlice(bootstrap))
	if len(bootstrapPeers) == 0 && bootstrap != nil {
		// Invalid bootstrap format, replace with "auto"
		confMap["Bootstrap"] = []string{config.AutoPlaceholder}
		return nil
	}

	// Process bootstrap peers according to migration rules
	newBootstrap := processBootstrapPeers(bootstrapPeers)
	confMap["Bootstrap"] = newBootstrap

	return nil
}

// processBootstrapPeers processes bootstrap peers according to migration rules
func processBootstrapPeers(peers []string) []string {
	// If empty, use "auto"
	if len(peers) == 0 {
		return []string{config.AutoPlaceholder}
	}

	// Filter out default peers to get only custom ones
	customPeers := slices.DeleteFunc(slices.Clone(peers), func(peer string) bool {
		return slices.Contains(DefaultBootstrapAddresses, peer)
	})

	// Check if any default peers were removed
	hasDefaultPeers := len(customPeers) < len(peers)

	// If we have default peers, replace them with "auto"
	if hasDefaultPeers {
		return append([]string{config.AutoPlaceholder}, customPeers...)
	}

	// No default peers found, keep as is
	return peers
}

// migrateDNSResolvers migrates DNS resolvers to use "auto" for "." eTLD
func migrateDNSResolvers(confMap map[string]any) error {
	// Get or create DNS section
	dns := common.GetOrCreateSection(confMap, "DNS")

	// Get existing resolvers or create empty map
	resolvers := common.SafeCastMap(dns["Resolvers"])

	// Define default resolvers that should be replaced with "auto"
	defaultResolvers := map[string]string{
		"https://dns.eth.limo/dns-query":                config.AutoPlaceholder,
		"https://dns.eth.link/dns-query":                config.AutoPlaceholder,
		"https://resolver.cloudflare-eth.com/dns-query": config.AutoPlaceholder,
	}

	// Replace default resolvers with "auto"
	stringResolvers := common.ReplaceDefaultsWithAuto(resolvers, defaultResolvers)

	// Ensure "." is set to "auto" if not already set
	if _, exists := stringResolvers["."]; !exists {
		stringResolvers["."] = config.AutoPlaceholder
	}

	dns["Resolvers"] = stringResolvers
	return nil
}

// migrateDelegatedRouters migrates DelegatedRouters to use "auto"
func migrateDelegatedRouters(confMap map[string]any) error {
	// Get or create Routing section
	routing := common.GetOrCreateSection(confMap, "Routing")

	// Get existing delegated routers
	delegatedRouters, exists := routing["DelegatedRouters"]

	// Check if it's empty or nil
	if !exists || common.IsEmptySlice(delegatedRouters) {
		routing["DelegatedRouters"] = []string{config.AutoPlaceholder}
		return nil
	}

	// Process the list to replace cid.contact with "auto" and preserve others
	routers := common.ConvertInterfaceSlice(common.SafeCastSlice(delegatedRouters))
	var newRouters []string
	hasAuto := false

	for _, router := range routers {
		if router == "https://cid.contact" {
			if !hasAuto {
				newRouters = append(newRouters, config.AutoPlaceholder)
				hasAuto = true
			}
		} else {
			newRouters = append(newRouters, router)
		}
	}

	// If empty after processing, add "auto"
	if len(newRouters) == 0 {
		newRouters = []string{config.AutoPlaceholder}
	}

	routing["DelegatedRouters"] = newRouters
	return nil
}

// migrateDelegatedPublishers migrates DelegatedPublishers to use "auto"
func migrateDelegatedPublishers(confMap map[string]any) error {
	// Get or create Ipns section
	ipns := common.GetOrCreateSection(confMap, "Ipns")

	// Get existing delegated publishers
	delegatedPublishers, exists := ipns["DelegatedPublishers"]

	// Check if it's empty or nil - only then replace with "auto"
	// Otherwise preserve custom publishers
	if !exists || common.IsEmptySlice(delegatedPublishers) {
		ipns["DelegatedPublishers"] = []string{config.AutoPlaceholder}
	}
	// If there are custom publishers, leave them as is

	return nil
}
