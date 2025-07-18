package mg16

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/config"
)

// Helper function to run migration on JSON input and return result
func runMigrationOnJSON(t *testing.T, input string) map[string]interface{} {
	t.Helper()
	var output bytes.Buffer
	// Use t.TempDir() for test isolation and parallel execution support
	tempDir := t.TempDir()
	err := convert(bytes.NewReader([]byte(input)), &output, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &result)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

// Helper function to assert nested map key has expected value
func assertMapKeyEquals(t *testing.T, result map[string]interface{}, path []string, key string, expected interface{}) {
	t.Helper()
	current := result
	for _, p := range path {
		section, exists := current[p]
		if !exists {
			t.Fatalf("Section %s not found in path %v", p, path)
		}
		current = section.(map[string]interface{})
	}

	if current[key] != expected {
		t.Errorf("Expected %s to be %v, got %v", key, expected, current[key])
	}
}

// Helper function to assert slice contains expected values
func assertSliceEquals(t *testing.T, result map[string]interface{}, path []string, expected []string) {
	t.Helper()
	current := result
	for i, p := range path[:len(path)-1] {
		section, exists := current[p]
		if !exists {
			t.Fatalf("Section %s not found in path %v at index %d", p, path, i)
		}
		current = section.(map[string]interface{})
	}

	sliceKey := path[len(path)-1]
	slice, exists := current[sliceKey]
	if !exists {
		t.Fatalf("Slice %s not found", sliceKey)
	}

	actualSlice := slice.([]interface{})
	if len(actualSlice) != len(expected) {
		t.Fatalf("Expected slice length %d, got %d", len(expected), len(actualSlice))
	}

	for i, exp := range expected {
		if actualSlice[i] != exp {
			t.Errorf("Expected slice[%d] to be %s, got %v", i, exp, actualSlice[i])
		}
	}
}

// Helper to build test config JSON with specified fields
func buildTestConfig(fields map[string]interface{}) string {
	config := map[string]interface{}{
		"Identity": map[string]interface{}{"PeerID": "QmTest"},
	}
	for k, v := range fields {
		config[k] = v
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	return string(data)
}

// Helper to run migration and get DNS resolvers
func runMigrationAndGetDNSResolvers(t *testing.T, input string) map[string]interface{} {
	t.Helper()
	result := runMigrationOnJSON(t, input)
	dns := result["DNS"].(map[string]interface{})
	return dns["Resolvers"].(map[string]interface{})
}

// Helper to assert multiple resolver values
func assertResolvers(t *testing.T, resolvers map[string]interface{}, expected map[string]string) {
	t.Helper()
	for key, expectedValue := range expected {
		if resolvers[key] != expectedValue {
			t.Errorf("Expected %s resolver to be %v, got %v", key, expectedValue, resolvers[key])
		}
	}
}

// =============================================================================
// End-to-End Migration Tests
// =============================================================================

func TestMigration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "migration-test-16-to-17")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test config with default bootstrap peers
	testConfig := map[string]interface{}{
		"Bootstrap": []string{
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
			"/ip4/192.168.1.1/tcp/4001/p2p/QmCustomPeer", // Custom peer
		},
		"DNS": map[string]interface{}{
			"Resolvers": map[string]string{},
		},
		"Routing": map[string]interface{}{
			"DelegatedRouters": []string{},
		},
		"Ipns": map[string]interface{}{
			"ResolveCacheSize": 128,
		},
		"Identity": map[string]interface{}{
			"PeerID": "QmTest",
		},
		"Version": map[string]interface{}{
			"Current": "0.36.0",
		},
	}

	// Write test config
	configPath := filepath.Join(tempDir, "config")
	configData, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(configPath, configData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create version file
	versionPath := filepath.Join(tempDir, "version")
	err = os.WriteFile(versionPath, []byte("16"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Run migration
	migration := &Migration{}
	opts := Options{
		Path:    tempDir,
		Verbose: true,
	}

	err = migration.Apply(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Verify version was updated
	versionData, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(versionData) != "17" {
		t.Errorf("Expected version 17, got %s", string(versionData))
	}

	// Verify config was updated
	configData, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var updatedConfig map[string]interface{}
	err = json.Unmarshal(configData, &updatedConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Check AutoConfig was added
	autoConfig, exists := updatedConfig["AutoConfig"]
	if !exists {
		t.Error("AutoConfig section not added")
	}
	autoConfigMap := autoConfig.(map[string]interface{})
	if autoConfigMap["URL"] != config.DefaultAutoConfigURL {
		t.Errorf("Expected AutoConfig URL %s, got %s", config.DefaultAutoConfigURL, autoConfigMap["URL"])
	}

	// Check Bootstrap was updated
	bootstrap := updatedConfig["Bootstrap"].([]interface{})
	if len(bootstrap) != 2 {
		t.Errorf("Expected 2 bootstrap entries, got %d", len(bootstrap))
	}
	if bootstrap[0] != "auto" {
		t.Errorf("Expected first bootstrap entry to be 'auto', got %s", bootstrap[0])
	}
	if bootstrap[1] != "/ip4/192.168.1.1/tcp/4001/p2p/QmCustomPeer" {
		t.Errorf("Expected custom peer to be preserved, got %s", bootstrap[1])
	}

	// Check DNS.Resolvers was updated
	dns := updatedConfig["DNS"].(map[string]interface{})
	resolvers := dns["Resolvers"].(map[string]interface{})
	if resolvers["."] != "auto" {
		t.Errorf("Expected DNS resolver for '.' to be 'auto', got %s", resolvers["."])
	}

	// Check Routing.DelegatedRouters was updated
	routing := updatedConfig["Routing"].(map[string]interface{})
	delegatedRouters := routing["DelegatedRouters"].([]interface{})
	if len(delegatedRouters) != 1 || delegatedRouters[0] != "auto" {
		t.Errorf("Expected DelegatedRouters to be ['auto'], got %v", delegatedRouters)
	}

	// Check Ipns.DelegatedPublishers was updated
	ipns := updatedConfig["Ipns"].(map[string]interface{})
	delegatedPublishers := ipns["DelegatedPublishers"].([]interface{})
	if len(delegatedPublishers) != 1 || delegatedPublishers[0] != "auto" {
		t.Errorf("Expected DelegatedPublishers to be ['auto'], got %v", delegatedPublishers)
	}

	// Test revert
	err = migration.Revert(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Verify version was reverted
	versionData, err = os.ReadFile(versionPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(versionData) != "16" {
		t.Errorf("Expected version 16 after revert, got %s", string(versionData))
	}
}

func TestConvert(t *testing.T) {
	t.Parallel()
	input := buildTestConfig(map[string]interface{}{
		"Bootstrap": []string{
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		},
	})

	result := runMigrationOnJSON(t, input)

	// Check that AutoConfig was added and enabled
	assertMapKeyEquals(t, result, []string{"AutoConfig"}, "Enabled", true)

	// Check that Bootstrap was updated to "auto"
	assertSliceEquals(t, result, []string{"Bootstrap"}, []string{"auto"})
}

// =============================================================================
// Bootstrap Migration Tests
// =============================================================================

func TestBootstrapMigration(t *testing.T) {
	t.Parallel()

	t.Run("process bootstrap peers logic verification", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name     string
			peers    []string
			expected []string
		}{
			{
				name:     "empty peers",
				peers:    []string{},
				expected: []string{"auto"},
			},
			{
				name: "only default peers",
				peers: []string{
					"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
					"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
				},
				expected: []string{"auto"},
			},
			{
				name: "mixed default and custom peers",
				peers: []string{
					"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
					"/ip4/192.168.1.1/tcp/4001/p2p/QmCustomPeer",
				},
				expected: []string{"auto", "/ip4/192.168.1.1/tcp/4001/p2p/QmCustomPeer"},
			},
			{
				name: "only custom peers",
				peers: []string{
					"/ip4/192.168.1.1/tcp/4001/p2p/QmCustomPeer1",
					"/ip4/192.168.1.2/tcp/4001/p2p/QmCustomPeer2",
				},
				expected: []string{
					"/ip4/192.168.1.1/tcp/4001/p2p/QmCustomPeer1",
					"/ip4/192.168.1.2/tcp/4001/p2p/QmCustomPeer2",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				result := processBootstrapPeers(tt.peers, "")
				if len(result) != len(tt.expected) {
					t.Errorf("Expected %d peers, got %d", len(tt.expected), len(result))
					return
				}
				for i, expected := range tt.expected {
					if result[i] != expected {
						t.Errorf("Expected peer %d to be %s, got %s", i, expected, result[i])
					}
				}
			})
		}
	})

	t.Run("replaces all old default bootstrapper peers with auto entry", func(t *testing.T) {
		t.Parallel()
		input := buildTestConfig(map[string]interface{}{
			"Bootstrap": []string{
				"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
				"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
				"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
				"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
				"/dnsaddr/va1.bootstrap.libp2p.io/p2p/12D3KooWKnDdG3iXw9eTFijk3EWSunZcFi54Zka4wmtqtt6rPxc8",
				"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
				"/ip4/104.131.131.82/udp/4001/quic-v1/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
			},
		})

		result := runMigrationOnJSON(t, input)
		assertSliceEquals(t, result, []string{"Bootstrap"}, []string{"auto"})
	})

	t.Run("creates Bootstrap section with auto when missing", func(t *testing.T) {
		t.Parallel()
		input := `{"Identity": {"PeerID": "QmTest"}}`
		result := runMigrationOnJSON(t, input)
		assertSliceEquals(t, result, []string{"Bootstrap"}, []string{"auto"})
	})
}

// =============================================================================
// DNS Migration Tests
// =============================================================================

func TestDNSMigration(t *testing.T) {
	t.Parallel()

	t.Run("creates DNS section with auto resolver when missing", func(t *testing.T) {
		t.Parallel()
		input := `{"Identity": {"PeerID": "QmTest"}}`
		result := runMigrationOnJSON(t, input)
		assertMapKeyEquals(t, result, []string{"DNS", "Resolvers"}, ".", "auto")
	})

	t.Run("preserves all custom DNS resolvers unchanged", func(t *testing.T) {
		t.Parallel()
		input := buildTestConfig(map[string]interface{}{
			"DNS": map[string]interface{}{
				"Resolvers": map[string]string{
					".":    "https://my-custom-resolver.com",
					".eth": "https://eth.resolver",
				},
			},
		})

		resolvers := runMigrationAndGetDNSResolvers(t, input)
		assertResolvers(t, resolvers, map[string]string{
			".":    "https://my-custom-resolver.com",
			".eth": "https://eth.resolver",
		})
	})

	t.Run("preserves custom dot and eth resolvers unchanged", func(t *testing.T) {
		t.Parallel()
		input := buildTestConfig(map[string]interface{}{
			"DNS": map[string]interface{}{
				"Resolvers": map[string]string{
					".":    "https://cloudflare-dns.com/dns-query",
					".eth": "https://example.com/dns-query",
				},
			},
		})

		resolvers := runMigrationAndGetDNSResolvers(t, input)
		assertResolvers(t, resolvers, map[string]string{
			".":    "https://cloudflare-dns.com/dns-query",
			".eth": "https://example.com/dns-query",
		})
	})

	t.Run("replaces old default eth resolver with auto", func(t *testing.T) {
		t.Parallel()
		input := buildTestConfig(map[string]interface{}{
			"DNS": map[string]interface{}{
				"Resolvers": map[string]string{
					".":       "https://cloudflare-dns.com/dns-query",
					".eth":    "https://dns.eth.limo/dns-query",                // should be replaced
					".crypto": "https://resolver.cloudflare-eth.com/dns-query", // should be replaced
					".link":   "https://dns.eth.link/dns-query",                // should be replaced
				},
			},
		})

		resolvers := runMigrationAndGetDNSResolvers(t, input)
		assertResolvers(t, resolvers, map[string]string{
			".":       "https://cloudflare-dns.com/dns-query", // preserved
			".eth":    "auto",                                 // replaced
			".crypto": "auto",                                 // replaced
			".link":   "auto",                                 // replaced
		})
	})
}

// =============================================================================
// Routing Migration Tests
// =============================================================================

func TestRoutingMigration(t *testing.T) {
	t.Parallel()

	t.Run("creates Routing section with auto DelegatedRouters when missing", func(t *testing.T) {
		t.Parallel()
		input := `{"Identity": {"PeerID": "QmTest"}}`
		result := runMigrationOnJSON(t, input)
		assertSliceEquals(t, result, []string{"Routing", "DelegatedRouters"}, []string{"auto"})
	})

	t.Run("replaces cid.contact with auto while preserving custom routers added by user", func(t *testing.T) {
		t.Parallel()
		input := buildTestConfig(map[string]interface{}{
			"Routing": map[string]interface{}{
				"DelegatedRouters": []string{
					"https://cid.contact",
					"https://my-custom-router.com",
				},
			},
		})

		result := runMigrationOnJSON(t, input)
		assertSliceEquals(t, result, []string{"Routing", "DelegatedRouters"}, []string{"auto", "https://my-custom-router.com"})
	})
}

// =============================================================================
// IPNS Migration Tests
// =============================================================================

func TestIpnsMigration(t *testing.T) {
	t.Parallel()

	t.Run("creates Ipns section with auto DelegatedPublishers when missing", func(t *testing.T) {
		t.Parallel()
		input := `{"Identity": {"PeerID": "QmTest"}}`
		result := runMigrationOnJSON(t, input)
		assertSliceEquals(t, result, []string{"Ipns", "DelegatedPublishers"}, []string{"auto"})
	})

	t.Run("preserves existing custom DelegatedPublishers unchanged", func(t *testing.T) {
		t.Parallel()
		input := buildTestConfig(map[string]interface{}{
			"Ipns": map[string]interface{}{
				"DelegatedPublishers": []string{
					"https://my-publisher.com",
					"https://another-publisher.com",
				},
			},
		})

		result := runMigrationOnJSON(t, input)
		assertSliceEquals(t, result, []string{"Ipns", "DelegatedPublishers"}, []string{"https://my-publisher.com", "https://another-publisher.com"})
	})

	t.Run("adds auto DelegatedPublishers to existing Ipns section", func(t *testing.T) {
		t.Parallel()
		input := buildTestConfig(map[string]interface{}{
			"Ipns": map[string]interface{}{
				"ResolveCacheSize": 128,
			},
		})

		result := runMigrationOnJSON(t, input)
		assertMapKeyEquals(t, result, []string{"Ipns"}, "ResolveCacheSize", float64(128))
		assertSliceEquals(t, result, []string{"Ipns", "DelegatedPublishers"}, []string{"auto"})
	})
}

// =============================================================================
// AutoConfig Migration Tests
// =============================================================================

func TestAutoConfigMigration(t *testing.T) {
	t.Parallel()

	t.Run("preserves existing AutoConfig fields unchanged", func(t *testing.T) {
		t.Parallel()
		input := buildTestConfig(map[string]interface{}{
			"AutoConfig": map[string]interface{}{
				"URL":         "https://custom.example.com/autoconfig.json",
				"Enabled":     false,
				"CustomField": "preserved",
			},
		})

		result := runMigrationOnJSON(t, input)
		assertMapKeyEquals(t, result, []string{"AutoConfig"}, "URL", "https://custom.example.com/autoconfig.json")
		assertMapKeyEquals(t, result, []string{"AutoConfig"}, "Enabled", false)
		assertMapKeyEquals(t, result, []string{"AutoConfig"}, "CustomField", "preserved")
	})
}
