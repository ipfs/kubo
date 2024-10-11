package config

import (
	"fmt"
	"net"
	"time"
)

// Transformer is a function which takes configuration and applies some filter to it.
type Transformer func(c *Config) error

// Profile contains the profile transformer the description of the profile.
type Profile struct {
	// Description briefly describes the functionality of the profile.
	Description string

	// Transform takes ipfs configuration and applies the profile to it.
	Transform Transformer

	// InitOnly specifies that this profile can only be applied on init.
	InitOnly bool
}

// defaultServerFilters has is a list of IPv4 and IPv6 prefixes that are private, local only, or unrouteable.
// according to https://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry.xhtml
// and https://www.iana.org/assignments/iana-ipv6-special-registry/iana-ipv6-special-registry.xhtml
var defaultServerFilters = []string{
	"/ip4/10.0.0.0/ipcidr/8",
	"/ip4/100.64.0.0/ipcidr/10",
	"/ip4/169.254.0.0/ipcidr/16",
	"/ip4/172.16.0.0/ipcidr/12",
	"/ip4/192.0.0.0/ipcidr/24",
	"/ip4/192.0.2.0/ipcidr/24",
	"/ip4/192.168.0.0/ipcidr/16",
	"/ip4/198.18.0.0/ipcidr/15",
	"/ip4/198.51.100.0/ipcidr/24",
	"/ip4/203.0.113.0/ipcidr/24",
	"/ip4/240.0.0.0/ipcidr/4",
	"/ip6/100::/ipcidr/64",
	"/ip6/2001:2::/ipcidr/48",
	"/ip6/2001:db8::/ipcidr/32",
	"/ip6/fc00::/ipcidr/7",
	"/ip6/fe80::/ipcidr/10",
}

// Profiles is a map holding configuration transformers. Docs are in docs/config.md.
var Profiles = map[string]Profile{
	"server": {
		Description: `Disables local host discovery, recommended when
running IPFS on machines with public IPv4 addresses.`,

		Transform: func(c *Config) error {
			c.Addresses.NoAnnounce = appendSingle(c.Addresses.NoAnnounce, defaultServerFilters)
			c.Swarm.AddrFilters = appendSingle(c.Swarm.AddrFilters, defaultServerFilters)
			c.Discovery.MDNS.Enabled = false
			c.Swarm.DisableNatPortMap = true
			return nil
		},
	},

	"local-discovery": {
		Description: `Sets default values to fields affected by the server
profile, enables discovery in local networks.`,

		Transform: func(c *Config) error {
			c.Addresses.NoAnnounce = deleteEntries(c.Addresses.NoAnnounce, defaultServerFilters)
			c.Swarm.AddrFilters = deleteEntries(c.Swarm.AddrFilters, defaultServerFilters)
			c.Discovery.MDNS.Enabled = true
			c.Swarm.DisableNatPortMap = false
			return nil
		},
	},
	"test": {
		Description: `Reduces external interference of IPFS daemon, this
is useful when using the daemon in test environments.`,

		Transform: func(c *Config) error {
			c.Addresses.API = Strings{"/ip4/127.0.0.1/tcp/0"}
			c.Addresses.Gateway = Strings{"/ip4/127.0.0.1/tcp/0"}
			c.Addresses.Swarm = []string{
				"/ip4/127.0.0.1/tcp/0",
			}

			c.Swarm.DisableNatPortMap = true
			c.Routing.LoopbackAddressesOnLanDHT = True

			c.Bootstrap = []string{}
			c.Discovery.MDNS.Enabled = false
			return nil
		},
	},
	"default-networking": {
		Description: `Restores default network settings.
Inverse profile of the test profile.`,

		Transform: func(c *Config) error {
			c.Addresses = addressesConfig()

			bootstrapPeers, err := DefaultBootstrapPeers()
			if err != nil {
				return err
			}
			c.Bootstrap = appendSingle(c.Bootstrap, BootstrapPeerStrings(bootstrapPeers))

			c.Swarm.DisableNatPortMap = false
			c.Discovery.MDNS.Enabled = true
			return nil
		},
	},
	"default-datastore": {
		Description: `Configures the node to use the default datastore (flatfs).

Read the "flatfs" profile description for more information on this datastore.

This profile may only be applied when first initializing the node.
`,

		InitOnly: true,
		Transform: func(c *Config) error {
			c.Datastore.Spec = flatfsSpec()
			return nil
		},
	},
	"flatfs": {
		Description: `Configures the node to use the flatfs datastore.

This is the most battle-tested and reliable datastore.
You should use this datastore if:

* You need a very simple and very reliable datastore, and you trust your
  filesystem. This datastore stores each block as a separate file in the
  underlying filesystem so it's unlikely to loose data unless there's an issue
  with the underlying file system.
* You need to run garbage collection in a way that reclaims free space as soon as possible.
* You want to minimize memory usage.
* You are ok with the default speed of data import, or prefer to use --nocopy.

See configuration documentation at:
https://github.com/ipfs/kubo/blob/master/docs/datastores.md#flatfs

NOTE: This profile may only be applied when first initializing node at IPFS_PATH
      via 'ipfs init --profile flatfs'
`,

		InitOnly: true,
		Transform: func(c *Config) error {
			c.Datastore.Spec = flatfsSpec()
			return nil
		},
	},
	"pebbleds": {
		Description: `Configures the node to use the pebble high-performance datastore.

Pebble is a LevelDB/RocksDB inspired key-value store focused on performance
and internal usage by CockroachDB.
You should use this datastore if:

- You need a datastore that is focused on performance.
- You need reliability by default, but may choose to disable WAL for maximum performance when reliability is not critical.
- This datastore is good for multi-terabyte data sets.
- May benefit from tuning depending on read/write patterns and throughput.
- Performance is helped significantly by running on a system with plenty of memory.

See configuration documentation at:
https://github.com/ipfs/kubo/blob/master/docs/datastores.md#pebbleds

NOTE: This profile may only be applied when first initializing node at IPFS_PATH
      via 'ipfs init --profile pebbleds'
`,

		InitOnly: true,
		Transform: func(c *Config) error {
			c.Datastore.Spec = pebbleSpec()
			return nil
		},
	},
	"badgerds": {
		Description: `Configures the node to use the legacy badgerv1 datastore.

NOTE: this is badger 1.x, which has known bugs and is no longer supported by the upstream team.
It is provided here only for pre-existing users, allowing them to migrate away to more modern datastore.

Other caveats:

* This datastore will not properly reclaim space when your datastore is
  smaller than several gigabytes.  If you run IPFS with --enable-gc, you plan
  on storing very little data in your IPFS node, and disk usage is more
  critical than performance, consider using flatfs.
* This datastore uses up to several gigabytes of memory.
* Good for medium-size datastores, but may run into performance issues
  if your dataset is bigger than a terabyte.

See configuration documentation at:
https://github.com/ipfs/kubo/blob/master/docs/datastores.md#badgerds

NOTE: This profile may only be applied when first initializing node at IPFS_PATH
      via 'ipfs init --profile badgerds'
`,

		InitOnly: true,
		Transform: func(c *Config) error {
			c.Datastore.Spec = badgerSpec()
			return nil
		},
	},
	"lowpower": {
		Description: `Reduces daemon overhead on the system. May affect node
functionality - performance of content discovery and data
fetching may be degraded.
`,
		Transform: func(c *Config) error {
			// Disable "server" services (dht, autonat, limited relay)
			c.Routing.Type = NewOptionalString("autoclient")
			c.AutoNAT.ServiceMode = AutoNATServiceDisabled
			c.Swarm.RelayService.Enabled = False

			// Keep bare minimum connections around
			lowWater := int64(20)
			highWater := int64(40)
			gracePeriod := time.Minute
			c.Swarm.ConnMgr.Type = NewOptionalString("basic")
			c.Swarm.ConnMgr.LowWater = &OptionalInteger{value: &lowWater}
			c.Swarm.ConnMgr.HighWater = &OptionalInteger{value: &highWater}
			c.Swarm.ConnMgr.GracePeriod = &OptionalDuration{&gracePeriod}
			return nil
		},
	},
	"announce-off": {
		Description: `Disables Reprovide system (and announcing to Amino DHT).

		USE WITH CAUTION:
		The main use case for this is setups with manual Peering.Peers config.
		Data from this node will not be announced on the DHT. This will make
		DHT-based routing an data retrieval impossible if this node is the only
		one hosting it, and other peers are not already connected to it.
`,
		Transform: func(c *Config) error {
			c.Reprovider.Interval = NewOptionalDuration(0) // 0 disables periodic reprovide
			c.Experimental.StrategicProviding = true       // this is not a typo (the name is counter-intuitive)
			return nil
		},
	},
	"announce-on": {
		Description: `Re-enables Reprovide system (reverts announce-off profile).`,
		Transform: func(c *Config) error {
			c.Reprovider.Interval = NewOptionalDuration(DefaultReproviderInterval) // have to apply explicit default because nil would be ignored
			c.Experimental.StrategicProviding = false                              // this is not a typo (the name is counter-intuitive)
			return nil
		},
	},
	"randomports": {
		Description: `Use a random port number for swarm.`,

		Transform: func(c *Config) error {
			port, err := getAvailablePort()
			if err != nil {
				return err
			}
			c.Addresses.Swarm = []string{
				fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
				fmt.Sprintf("/ip6/::/tcp/%d", port),
			}
			return nil
		},
	},
	"legacy-cid-v0": {
		Description: `Makes UnixFS import produce legacy CIDv0 with no raw leaves, sha2-256 and 256 KiB chunks.`,

		Transform: func(c *Config) error {
			c.Import.CidVersion = *NewOptionalInteger(0)
			c.Import.UnixFSRawLeaves = False
			c.Import.UnixFSChunker = *NewOptionalString("size-262144")
			c.Import.HashFunction = *NewOptionalString("sha2-256")
			return nil
		},
	},
	"test-cid-v1": {
		Description: `Makes UnixFS import produce modern CIDv1 with raw leaves, sha2-256 and 1 MiB chunks.`,

		Transform: func(c *Config) error {
			c.Import.CidVersion = *NewOptionalInteger(1)
			c.Import.UnixFSRawLeaves = True
			c.Import.UnixFSChunker = *NewOptionalString("size-1048576")
			c.Import.HashFunction = *NewOptionalString("sha2-256")
			return nil
		},
	},
}

func getAvailablePort() (port int, err error) {
	ln, err := net.Listen("tcp", "[::]:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	port = ln.Addr().(*net.TCPAddr).Port
	return port, nil
}

func appendSingle(a []string, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	m := map[string]bool{}
	for _, f := range a {
		if !m[f] {
			out = append(out, f)
		}
		m[f] = true
	}
	for _, f := range b {
		if !m[f] {
			out = append(out, f)
		}
		m[f] = true
	}
	return out
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
