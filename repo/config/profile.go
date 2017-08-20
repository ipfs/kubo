package config

// Transformer is a function which takes configuration and applies some filter to it
type Transformer func(c *Config) error

// defaultServerFilters has a list of non-routable IPv4 prefixes
// according to http://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry.xhtml
var defaultServerFilters = []string{
	"/ip4/10.0.0.0/ipcidr/8",
	"/ip4/100.64.0.0/ipcidr/10",
	"/ip4/169.254.0.0/ipcidr/16",
	"/ip4/172.16.0.0/ipcidr/12",
	"/ip4/192.0.0.0/ipcidr/24",
	"/ip4/192.0.0.0/ipcidr/29",
	"/ip4/192.0.0.8/ipcidr/32",
	"/ip4/192.0.0.170/ipcidr/32",
	"/ip4/192.0.0.171/ipcidr/32",
	"/ip4/192.0.2.0/ipcidr/24",
	"/ip4/192.168.0.0/ipcidr/16",
	"/ip4/198.18.0.0/ipcidr/15",
	"/ip4/198.51.100.0/ipcidr/24",
	"/ip4/203.0.113.0/ipcidr/24",
	"/ip4/240.0.0.0/ipcidr/4",
}

// Profiles is a map holding configuration transformers. Docs are in docs/config.md
var Profiles = map[string]Transformer{
	"server": func(c *Config) error {
		c.Addresses.NoAnnounce = appendSingle(c.Addresses.NoAnnounce, defaultServerFilters)
		c.Swarm.AddrFilters = appendSingle(c.Swarm.AddrFilters, defaultServerFilters)
		c.Discovery.MDNS.Enabled = false
		return nil
	},
	"local-discovery": func(c *Config) error {
		c.Addresses.NoAnnounce = deleteEntries(c.Addresses.NoAnnounce, defaultServerFilters)
		c.Swarm.AddrFilters = deleteEntries(c.Swarm.AddrFilters, defaultServerFilters)
		c.Discovery.MDNS.Enabled = true
		return nil
	},
	"test": func(c *Config) error {
		c.Addresses.API = "/ip4/127.0.0.1/tcp/0"
		c.Addresses.Gateway = "/ip4/127.0.0.1/tcp/0"
		c.Addresses.Swarm = []string{
			"/ip4/127.0.0.1/tcp/0",
		}

		c.Swarm.DisableNatPortMap = true

		c.Bootstrap = []string{}
		c.Discovery.MDNS.Enabled = false
		return nil
	},
	"default-networking": func(c *Config) error {
		c.Addresses = addressesConfig()

		c.Swarm.DisableNatPortMap = false
		c.Discovery.MDNS.Enabled = true
		return nil
	},
	"badgerds": func(c *Config) error {
		c.Datastore.Spec = map[string]interface{}{
			"type":   "measure",
			"prefix": "badger.datastore",
			"child": map[string]interface{}{
				"type":       "badgerds",
				"path":       "badgerds",
				"syncWrites": true,
			},
		}
		return nil
	},
	"default-datastore": func(c *Config) error {
		c.Datastore.Spec = DefaultDatastoreConfig().Spec
		return nil
	},
	"lowpower": func(c *Config) error {
		c.Discovery.Routing = "dhtclient"
		c.Reprovider.Interval = "0"
		return nil
	},
}

func appendSingle(a []string, b []string) []string {
	m := map[string]struct{}{}
	for _, f := range a {
		m[f] = struct{}{}
	}
	for _, f := range b {
		m[f] = struct{}{}
	}
	return mapKeys(m)
}

func deleteEntries(arr []string, del []string) []string {
	m := map[string]struct{}{}
	for _, f := range arr {
		m[f] = struct{}{}
	}
	for _, f := range del {
		delete(m, f)
	}
	return mapKeys(m)
}

func mapKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for f := range m {
		out = append(out, f)
	}
	return out
}
