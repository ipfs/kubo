package node

import (
	"fmt"
	"time"

	"github.com/ipfs/boxo/ipns"
	util "github.com/ipfs/boxo/util"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peerstore"
	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/boxo/namesys/republisher"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
)

const DefaultIpnsCacheSize = 128

// RecordValidator provides namesys compatible routing record validator
func RecordValidator(ps peerstore.Peerstore) record.Validator {
	return record.NamespacedValidator{
		"pk":   record.PublicKeyValidator{},
		"ipns": ipns.Validator{KeyBook: ps},
	}
}

// Namesys creates new name system
func Namesys(cacheSize int, cacheMaxTTL time.Duration) func(rt irouting.ProvideManyRouter, rslv *madns.Resolver, repo repo.Repo) (namesys.NameSystem, error) {
	return func(rt irouting.ProvideManyRouter, rslv *madns.Resolver, repo repo.Repo) (namesys.NameSystem, error) {
		opts := []namesys.Option{
			namesys.WithDatastore(repo.Datastore()),
			namesys.WithDNSResolver(rslv),
			namesys.WithMaxCacheTTL(cacheMaxTTL),
		}

		if cacheSize > 0 {
			opts = append(opts, namesys.WithCache(cacheSize))
		}

		return namesys.NewNameSystem(rt, opts...)
	}
}

// IpnsRepublisher runs new IPNS republisher service
func IpnsRepublisher(repubPeriod time.Duration, recordLifetime time.Duration) func(lcStartStop, namesys.NameSystem, repo.Repo, crypto.PrivKey) error {
	return func(lc lcStartStop, namesys namesys.NameSystem, repo repo.Repo, privKey crypto.PrivKey) error {
		repub := republisher.NewRepublisher(namesys, repo.Datastore(), privKey, repo.Keystore())

		if repubPeriod != 0 {
			if !util.Debug && (repubPeriod < time.Minute || repubPeriod > (time.Hour*24)) {
				return fmt.Errorf("config setting IPNS.RepublishPeriod is not between 1min and 1day: %s", repubPeriod)
			}

			repub.Interval = repubPeriod
		}

		if recordLifetime != 0 {
			repub.RecordLifetime = recordLifetime
		}

		lc.Append(repub.Run)
		return nil
	}
}
