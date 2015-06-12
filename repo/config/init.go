package config

import "io"

func Init(out io.Writer) (*Config, error) {
	ds, err := datastoreConfig()
	if err != nil {
		return nil, err
	}

	bootstrapPeers, err := DefaultBootstrapPeers()
	if err != nil {
		return nil, err
	}

	snr, err := initSNRConfig()
	if err != nil {
		return nil, err
	}

	conf := &Config{

		// setup the node's default addresses.
		// Note: two swarm listen addrs, one tcp, one utp.
		Addresses: Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				// "/ip4/0.0.0.0/udp/4002/utp", // disabled for now.
			},
			API:     "/ip4/127.0.0.1/tcp/5001",
			Gateway: "/ip4/127.0.0.1/tcp/8080",
		},

		Bootstrap:        BootstrapPeerStrings(bootstrapPeers),
		SupernodeRouting: *snr,
		Datastore:        *ds,
		Discovery: Discovery{MDNS{
			Enabled:  true,
			Interval: 10,
		}},
		Log: Log{
			MaxSizeMB:  250,
			MaxBackups: 1,
		},

		// setup the node mount points.
		Mounts: Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		// tracking ipfs version used to generate the init folder and adding
		// update checker default setting.
		Version: VersionDefaultValue(),

		Gateway: Gateway{
			RootRedirect: "",
			Writable:     false,
		},
	}

	return conf, nil
}

func datastoreConfig() (*Datastore, error) {
	dspath, err := DataStorePath("")
	if err != nil {
		return nil, err
	}
	return &Datastore{
		Path: dspath,
		Type: "leveldb",
	}, nil
}
