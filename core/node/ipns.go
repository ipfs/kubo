package node

import (
	"fmt"
	"time"

	"github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs-util"
	"github.com/ipfs/go-ipns"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p-routing"

	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/namesys/republisher"
	"github.com/ipfs/go-ipfs/repo"
)

const DefaultIpnsCacheSize = 128

func RecordValidator(ps peerstore.Peerstore) record.Validator {
	return record.NamespacedValidator{
		"pk":   record.PublicKeyValidator{},
		"ipns": ipns.Validator{KeyBook: ps},
	}
}

func OfflineNamesysCtor(rt routing.IpfsRouting, repo repo.Repo) (namesys.NameSystem, error) {
	return namesys.NewNameSystem(rt, repo.Datastore(), 0), nil
}

func OnlineNamesysCtor(rt routing.IpfsRouting, repo repo.Repo, cfg *config.Config) (namesys.NameSystem, error) {
	cs := cfg.Ipns.ResolveCacheSize
	if cs == 0 {
		cs = DefaultIpnsCacheSize
	}
	if cs < 0 {
		return nil, fmt.Errorf("cannot specify negative resolve cache size")
	}
	return namesys.NewNameSystem(rt, repo.Datastore(), cs), nil
}

func IpnsRepublisher(lc lcProcess, cfg *config.Config, namesys namesys.NameSystem, repo repo.Repo, privKey crypto.PrivKey) error {
	repub := republisher.NewRepublisher(namesys, repo.Datastore(), privKey, repo.Keystore())

	if cfg.Ipns.RepublishPeriod != "" {
		d, err := time.ParseDuration(cfg.Ipns.RepublishPeriod)
		if err != nil {
			return fmt.Errorf("failure to parse config setting IPNS.RepublishPeriod: %s", err)
		}

		if !util.Debug && (d < time.Minute || d > (time.Hour*24)) {
			return fmt.Errorf("config setting IPNS.RepublishPeriod is not between 1min and 1day: %s", d)
		}

		repub.Interval = d
	}

	if cfg.Ipns.RecordLifetime != "" {
		d, err := time.ParseDuration(cfg.Ipns.RecordLifetime)
		if err != nil {
			return fmt.Errorf("failure to parse config setting IPNS.RecordLifetime: %s", err)
		}

		repub.RecordLifetime = d
	}

	lc.Append(repub.Run)
	return nil
}
