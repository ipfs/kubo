package config

import (
	"testing"
)

func TestClone(t *testing.T) {
	c := new(Config)
	c.Identity.PeerID = "faketest"
	c.API.HTTPHeaders = map[string][]string{"foo": {"bar"}}

	newCfg, err := c.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if newCfg.Identity.PeerID != c.Identity.PeerID {
		t.Fatal("peer ID not preserved")
	}

	c.API.HTTPHeaders["foo"] = []string{"baz"}
	if newCfg.API.HTTPHeaders["foo"][0] != "bar" {
		t.Fatal("HTTP headers not preserved")
	}

	delete(c.API.HTTPHeaders, "foo")
	if newCfg.API.HTTPHeaders["foo"][0] != "bar" {
		t.Fatal("HTTP headers not preserved")
	}
}

func TestReflectToMap(t *testing.T) {
	// Helper function to create a test config with various field types
	reflectedConfig := ReflectToMap(new(Config))

	mapConfig, ok := reflectedConfig.(map[string]interface{})
	if !ok {
		t.Fatal("Config didn't convert to map")
	}

	reflectedIdentity, ok := mapConfig["Identity"]
	if !ok {
		t.Fatal("Identity field not found")
	}

	mapIdentity, ok := reflectedIdentity.(map[string]interface{})
	if !ok {
		t.Fatal("Identity field didn't convert to map")
	}

	// Test string field reflection
	reflectedPeerID, ok := mapIdentity["PeerID"]
	if !ok {
		t.Fatal("PeerID field not found in Identity")
	}
	if _, ok := reflectedPeerID.(string); !ok {
		t.Fatal("PeerID field didn't convert to string")
	}

	// Test omitempty json string field
	reflectedPrivKey, ok := mapIdentity["PrivKey"]
	if !ok {
		t.Fatal("PrivKey omitempty field not found in Identity")
	}
	if _, ok := reflectedPrivKey.(string); !ok {
		t.Fatal("PrivKey omitempty field didn't convert to string")
	}

	// Test slices field
	reflectedBootstrap, ok := mapConfig["Bootstrap"]
	if !ok {
		t.Fatal("Bootstrap field not found in config")
	}
	bootstrap, ok := reflectedBootstrap.([]interface{})
	if !ok {
		t.Fatal("Bootstrap field didn't convert to []string")
	}
	if len(bootstrap) != 0 {
		t.Fatal("Bootstrap len is incorrect")
	}

	reflectedDatastore, ok := mapConfig["Datastore"]
	if !ok {
		t.Fatal("Datastore field not found in config")
	}
	datastore, ok := reflectedDatastore.(map[string]interface{})
	if !ok {
		t.Fatal("Datastore field didn't convert to map")
	}
	storageGCWatermark, ok := datastore["StorageGCWatermark"]
	if !ok {
		t.Fatal("StorageGCWatermark field not found in Datastore")
	}
	// Test int field
	if _, ok := storageGCWatermark.(int64); !ok {
		t.Fatal("StorageGCWatermark field didn't convert to int64")
	}
	noSync, ok := datastore["NoSync"]
	if !ok {
		t.Fatal("NoSync field not found in Datastore")
	}
	// Test bool field
	if _, ok := noSync.(bool); !ok {
		t.Fatal("NoSync field didn't convert to bool")
	}

	reflectedDNS, ok := mapConfig["DNS"]
	if !ok {
		t.Fatal("DNS field not found in config")
	}
	DNS, ok := reflectedDNS.(map[string]interface{})
	if !ok {
		t.Fatal("DNS field didn't convert to map")
	}
	reflectedResolvers, ok := DNS["Resolvers"]
	if !ok {
		t.Fatal("Resolvers field not found in DNS")
	}
	// Test map field
	if _, ok := reflectedResolvers.(map[string]interface{}); !ok {
		t.Fatal("Resolvers field didn't convert to map")
	}

	// Test pointer field
	if _, ok := DNS["MaxCacheTTL"].(map[string]interface{}); !ok {
		// Since OptionalDuration only field is private, we cannot test it
		t.Fatal("MaxCacheTTL field didn't convert to map")
	}
}

// Test validation of options set through "ipfs config"
func TestCheckKey(t *testing.T) {
	err := CheckKey("Foo.Bar")
	if err == nil {
		t.Fatal("Foo.Bar isn't a valid key in the config")
	}

	err = CheckKey("Provider.Strategy")
	if err != nil {
		t.Fatalf("%s: %s", err, "Provider.Strategy is a valid key in the config")
	}

	err = CheckKey("Provider.Foo")
	if err == nil {
		t.Fatal("Provider.Foo isn't a valid key in the config")
	}

	err = CheckKey("Gateway.PublicGateways.Foo.Paths")
	if err != nil {
		t.Fatalf("%s: %s", err, "Gateway.PublicGateways.Foo.Paths is a valid key in the config")
	}

	err = CheckKey("Gateway.PublicGateways.Foo.Bar")
	if err == nil {
		t.Fatal("Gateway.PublicGateways.Foo.Bar isn't a valid key in the config")
	}

	err = CheckKey("Plugins.Plugins.peerlog.Config.Enabled")
	if err != nil {
		t.Fatalf("%s: %s", err, "Plugins.Plugins.peerlog.Config.Enabled is a valid key in the config")
	}
}
