// package mg16 contains the code to perform 16-17 repository migration in Kubo.
// This handles the following:
// - Migrate default bootstrap peers to "auto"
// - Migrate DNS resolvers to use "auto" for "." eTLD
// - Enable AutoConf system with default settings
// - Increment repo version to 17
package mg16

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/repo/fsrepo/migrations/atomicfile"
)

// Options contains migration options for embedded migrations
type Options struct {
	Path    string
	Verbose bool
}

const backupSuffix = ".16-to-17.bak"

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

// Migration implements the migration described above.
type Migration struct{}

// Versions returns the current version string for this migration.
func (m Migration) Versions() string {
	return "16-to-17"
}

// Reversible returns true, as we keep old config around
func (m Migration) Reversible() bool {
	return true
}

// Apply update the config.
func (m Migration) Apply(opts Options) error {
	if opts.Verbose {
		fmt.Printf("applying %s repo migration\n", m.Versions())
	}

	// Check version
	if err := checkVersion(opts.Path, "16"); err != nil {
		return err
	}

	if opts.Verbose {
		fmt.Println("> Upgrading config to use AutoConf system")
	}

	path := filepath.Join(opts.Path, "config")
	in, err := os.Open(path)
	if err != nil {
		return err
	}

	// make backup
	backup, err := atomicfile.New(path+backupSuffix, 0600)
	if err != nil {
		return err
	}
	if _, err := backup.ReadFrom(in); err != nil {
		panicOnError(backup.Abort())
		return err
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		panicOnError(backup.Abort())
		return err
	}

	// Create a temp file to write the output to on success
	out, err := atomicfile.New(path, 0600)
	if err != nil {
		panicOnError(backup.Abort())
		panicOnError(in.Close())
		return err
	}

	if err := convert(in, out, opts.Path); err != nil {
		panicOnError(out.Abort())
		panicOnError(backup.Abort())
		panicOnError(in.Close())
		return err
	}

	if err := in.Close(); err != nil {
		panicOnError(out.Abort())
		panicOnError(backup.Abort())
	}

	if err := writeVersion(opts.Path, "17"); err != nil {
		fmt.Println("failed to update version file to 17")
		// There was an error so abort writing the output and clean up temp file
		panicOnError(out.Abort())
		panicOnError(backup.Abort())
		return err
	} else {
		// Write the output and clean up temp file
		panicOnError(out.Close())
		panicOnError(backup.Close())
	}

	if opts.Verbose {
		fmt.Println("updated version file")
		fmt.Println("Migration 16 to 17 succeeded")
	}
	return nil
}

// panicOnError is reserved for checks we can't solve transactionally if an error occurs
func panicOnError(e error) {
	if e != nil {
		panic(fmt.Errorf("error can't be dealt with transactionally: %w", e))
	}
}

func (m Migration) Revert(opts Options) error {
	if opts.Verbose {
		fmt.Println("reverting migration")
	}

	if err := checkVersion(opts.Path, "17"); err != nil {
		return err
	}

	cfg := filepath.Join(opts.Path, "config")
	if err := os.Rename(cfg+backupSuffix, cfg); err != nil {
		return err
	}

	if err := writeVersion(opts.Path, "16"); err != nil {
		return err
	}
	if opts.Verbose {
		fmt.Println("lowered version number to 16")
	}

	return nil
}

// checkVersion verifies the repo is at the expected version
func checkVersion(repoPath string, expectedVersion string) error {
	versionPath := filepath.Join(repoPath, "version")
	versionBytes, err := os.ReadFile(versionPath)
	if err != nil {
		return fmt.Errorf("could not read version file: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	if version != expectedVersion {
		return fmt.Errorf("expected version %s, got %s", expectedVersion, version)
	}
	return nil
}

// writeVersion writes the version to the repo
func writeVersion(repoPath string, version string) error {
	versionPath := filepath.Join(repoPath, "version")
	return os.WriteFile(versionPath, []byte(version), 0644)
}

// convert converts the config from version 16 to 17
func convert(in io.Reader, out io.Writer, repoPath string) error {
	confMap := make(map[string]any)
	if err := json.NewDecoder(in).Decode(&confMap); err != nil {
		return err
	}

	// Enable AutoConf system
	if err := enableAutoConf(confMap); err != nil {
		return err
	}

	// Migrate Bootstrap peers
	if err := migrateBootstrap(confMap, repoPath); err != nil {
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
	fixed, err := json.MarshalIndent(confMap, "", "  ")
	if err != nil {
		return err
	}

	if _, err := out.Write(fixed); err != nil {
		return err
	}
	_, err = out.Write([]byte("\n"))
	return err
}

// enableAutoConf adds AutoConf section to config
func enableAutoConf(confMap map[string]any) error {
	// Check if AutoConf already exists
	if _, exists := confMap["AutoConf"]; exists {
		return nil
	}

	// Add empty AutoConf section - all fields will use implicit defaults:
	// - Enabled defaults to true (via DefaultAutoConfEnabled)
	// - URL defaults to mainnet URL (via DefaultAutoConfURL)
	// - RefreshInterval defaults to 24h (via DefaultAutoConfRefreshInterval)
	// - TLSInsecureSkipVerify defaults to false (no WithDefault, but false is zero value)
	confMap["AutoConf"] = map[string]any{}

	return nil
}

// migrateBootstrap migrates bootstrap peers to use "auto"
func migrateBootstrap(confMap map[string]any, repoPath string) error {
	bootstrap, exists := confMap["Bootstrap"]
	if !exists {
		// No bootstrap section, add "auto"
		confMap["Bootstrap"] = []string{"auto"}
		return nil
	}

	bootstrapSlice, ok := bootstrap.([]interface{})
	if !ok {
		// Invalid bootstrap format, replace with "auto"
		confMap["Bootstrap"] = []string{"auto"}
		return nil
	}

	// Convert to string slice
	var bootstrapPeers []string
	for _, peer := range bootstrapSlice {
		if peerStr, ok := peer.(string); ok {
			bootstrapPeers = append(bootstrapPeers, peerStr)
		}
	}

	// Check if we should replace with "auto"
	newBootstrap := processBootstrapPeers(bootstrapPeers, repoPath)
	confMap["Bootstrap"] = newBootstrap

	return nil
}

// processBootstrapPeers processes bootstrap peers according to migration rules
func processBootstrapPeers(peers []string, repoPath string) []string {
	// If empty, use "auto"
	if len(peers) == 0 {
		return []string{"auto"}
	}

	// Separate default peers from custom ones
	var customPeers []string
	var hasDefaultPeers bool

	for _, peer := range peers {
		if slices.Contains(DefaultBootstrapAddresses, peer) {
			hasDefaultPeers = true
		} else {
			customPeers = append(customPeers, peer)
		}
	}

	// If we have default peers, replace them with "auto"
	if hasDefaultPeers {
		return append([]string{"auto"}, customPeers...)
	}

	// No default peers found, keep as is
	return peers
}

// migrateDNSResolvers migrates DNS resolvers to use "auto" for "." eTLD
func migrateDNSResolvers(confMap map[string]any) error {
	dnsSection, exists := confMap["DNS"]
	if !exists {
		// No DNS section, create it with "auto"
		confMap["DNS"] = map[string]any{
			"Resolvers": map[string]string{
				".": config.AutoPlaceholder,
			},
		}
		return nil
	}

	dns, ok := dnsSection.(map[string]any)
	if !ok {
		// Invalid DNS format, replace with "auto"
		confMap["DNS"] = map[string]any{
			"Resolvers": map[string]string{
				".": config.AutoPlaceholder,
			},
		}
		return nil
	}

	resolvers, exists := dns["Resolvers"]
	if !exists {
		// No resolvers, add "auto"
		dns["Resolvers"] = map[string]string{
			".": config.AutoPlaceholder,
		}
		return nil
	}

	resolversMap, ok := resolvers.(map[string]any)
	if !ok {
		// Invalid resolvers format, replace with "auto"
		dns["Resolvers"] = map[string]string{
			".": config.AutoPlaceholder,
		}
		return nil
	}

	// Convert to string map and replace default resolvers with "auto"
	stringResolvers := make(map[string]string)
	defaultResolvers := map[string]string{
		"https://dns.eth.limo/dns-query":                "auto",
		"https://dns.eth.link/dns-query":                "auto",
		"https://resolver.cloudflare-eth.com/dns-query": "auto",
	}

	for k, v := range resolversMap {
		if vStr, ok := v.(string); ok {
			// Check if this is a default resolver that should be replaced
			if replacement, isDefault := defaultResolvers[vStr]; isDefault {
				stringResolvers[k] = replacement
			} else {
				stringResolvers[k] = vStr
			}
		}
	}

	// If "." is not set or empty, set it to "auto"
	if _, exists := stringResolvers["."]; !exists {
		stringResolvers["."] = "auto"
	}

	dns["Resolvers"] = stringResolvers
	return nil
}

// migrateDelegatedRouters migrates DelegatedRouters to use "auto"
func migrateDelegatedRouters(confMap map[string]any) error {
	routing, exists := confMap["Routing"]
	if !exists {
		// No routing section, create it with "auto"
		confMap["Routing"] = map[string]any{
			"DelegatedRouters": []string{"auto"},
		}
		return nil
	}

	routingMap, ok := routing.(map[string]any)
	if !ok {
		// Invalid routing format, replace with "auto"
		confMap["Routing"] = map[string]any{
			"DelegatedRouters": []string{"auto"},
		}
		return nil
	}

	delegatedRouters, exists := routingMap["DelegatedRouters"]
	if !exists {
		// No delegated routers, add "auto"
		routingMap["DelegatedRouters"] = []string{"auto"}
		return nil
	}

	// Check if it's empty or nil
	if shouldReplaceWithAuto(delegatedRouters) {
		routingMap["DelegatedRouters"] = []string{"auto"}
		return nil
	}

	// Process the list to replace cid.contact with "auto" and preserve others
	if slice, ok := delegatedRouters.([]interface{}); ok {
		var newRouters []string
		hasAuto := false

		for _, router := range slice {
			if routerStr, ok := router.(string); ok {
				if routerStr == "https://cid.contact" {
					if !hasAuto {
						newRouters = append(newRouters, "auto")
						hasAuto = true
					}
				} else {
					newRouters = append(newRouters, routerStr)
				}
			}
		}

		// If empty after processing, add "auto"
		if len(newRouters) == 0 {
			newRouters = []string{"auto"}
		}

		routingMap["DelegatedRouters"] = newRouters
	}

	return nil
}

// migrateDelegatedPublishers migrates DelegatedPublishers to use "auto"
func migrateDelegatedPublishers(confMap map[string]any) error {
	ipns, exists := confMap["Ipns"]
	if !exists {
		// No IPNS section, create it with "auto"
		confMap["Ipns"] = map[string]any{
			"DelegatedPublishers": []string{"auto"},
		}
		return nil
	}

	ipnsMap, ok := ipns.(map[string]any)
	if !ok {
		// Invalid IPNS format, replace with "auto"
		confMap["Ipns"] = map[string]any{
			"DelegatedPublishers": []string{"auto"},
		}
		return nil
	}

	delegatedPublishers, exists := ipnsMap["DelegatedPublishers"]
	if !exists {
		// No delegated publishers, add "auto"
		ipnsMap["DelegatedPublishers"] = []string{"auto"}
		return nil
	}

	// Check if it's empty or nil - only then replace with "auto"
	// Otherwise preserve custom publishers
	if shouldReplaceWithAuto(delegatedPublishers) {
		ipnsMap["DelegatedPublishers"] = []string{"auto"}
	}
	// If there are custom publishers, leave them as is

	return nil
}

// shouldReplaceWithAuto checks if a field should be replaced with "auto"
func shouldReplaceWithAuto(field any) bool {
	// If it's nil, replace with "auto"
	if field == nil {
		return true
	}

	// If it's an empty slice, replace with "auto"
	if slice, ok := field.([]interface{}); ok {
		return len(slice) == 0
	}

	// If it's an empty array, replace with "auto"
	if reflect.TypeOf(field).Kind() == reflect.Slice {
		v := reflect.ValueOf(field)
		return v.Len() == 0
	}

	return false
}
