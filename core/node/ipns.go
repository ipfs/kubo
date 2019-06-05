package node

import (
	"fmt"
	"time"

	"github.com/ipfs/go-ipfs-util"
	"github.com/ipfs/go-ipns"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/libp2p/go-libp2p-record"

	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/namesys/republisher"
	"github.com/ipfs/go-ipfs/repo"
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
func Namesys(cacheSize int) func(rt routing.Routing, repo repo.Repo) (namesys.NameSystem, error) {
	return func(rt routing.Routing, repo repo.Repo) (namesys.NameSystem, error) {
		return namesys.NewNameSystem(rt, repo.Datastore(), cacheSize), nil
	}
}

// IpnsRepublisher runs new IPNS republisher service
func IpnsRepublisher(repubPeriod time.Duration, recordLifetime time.Duration) func(lcProcess, namesys.NameSystem, repo.Repo, crypto.PrivKey) error {
	return func(lc lcProcess, namesys namesys.NameSystem, repo repo.Repo, privKey crypto.PrivKey) error {
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
