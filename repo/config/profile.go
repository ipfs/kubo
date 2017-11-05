package config

// Transformer is a function which takes configuration and applies some filter to it
type Transformer func(c *Config) error

// Profile applies some set of changes to the configuration
type Profile struct {
	Apply  Transformer
	Revert Transformer
}

// Profiles is a map holding configuration transformers. Docs are in docs/config.md
var Profiles = map[string]*Profile{
	"server": {
		Apply: func(c *Config) error {

			// defaultServerFilters has a list of non-routable IPv4 prefixes
			// according to http://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry.xhtml
			defaultServerFilters := []string{
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

			c.Addresses.NoAnnounce = append(c.Addresses.NoAnnounce, defaultServerFilters...)
			c.Swarm.AddrFilters = append(c.Swarm.AddrFilters, defaultServerFilters...)
			c.Discovery.MDNS.Enabled = false
			return nil
		},
		Revert: func(c *Config) error {
			c.Addresses.NoAnnounce = []string{}
			c.Swarm.AddrFilters = []string{}
			c.Discovery.MDNS.Enabled = true
			return nil
		},
	},
	"test": {
		Apply: func(c *Config) error {
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
		Revert: func(c *Config) error {
			c.Addresses = addressesConfig()

			c.Swarm.DisableNatPortMap = false
			return nil
		},
	},
	"badgerds": {
		Apply: func(c *Config) error {
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
		Revert: func(c *Config) error {
			c.Datastore.Spec = DefaultDatastoreConfig().Spec
			return nil
		},
	},
}
