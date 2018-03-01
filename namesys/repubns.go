package namesys

import (
	"context"
	"time"

	path "github.com/ipfs/go-ipfs/path"

	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	routing "gx/ipfs/QmTiWLZ6Fo5j4KcTVutZJ5KWRRJrbxzmxA4td8NfEdrPh7/go-libp2p-routing"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	dshelp "gx/ipfs/QmdQTPWduSeyveSxeCAte33M592isSW5Z979g81aJphrgn/go-ipfs-ds-help"
)

// repubns extends mpns to store published records in the datastore so
// that they will be picked up by the republisher
type repubns struct {
	mpns
	dstore ds.Datastore
}

// NewRepublishingNameSystem will construct an extension of the naming system that
// also republishes nodes
func NewRepublishingNameSystem(r routing.ValueStore, ds ds.Datastore, cachesize int) NameSystem {
	ns := NewNameSystem(r, ds, cachesize)
	return &repubns{*ns.(*mpns), ds}
}

// Publish publishes a name / value
func (ns *repubns) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return ns.PublishWithEOL(ctx, name, value, time.Now().Add(DefaultRecordTTL))
}

// Publish publishes a name / value with an EOL
func (ns *repubns) PublishWithEOL(ctx context.Context, name ci.PrivKey, value path.Path, eol time.Time) error {
	err := ns.mpns.PublishWithEOL(ctx, name, value, eol)
	if err != nil {
		return err
	}

	id, err := peer.IDFromPrivateKey(name)
	if err != nil {
		return err
	}

	_, ipnskey := IpnsKeysForID(id)
	rec, err := ns.dstore.Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
	if err != nil {
		log.Errorf("Republisher could not retrieve record from datastore that was just published")
		return err
	}

	repubkey := RepubKeyForID(id)
	log.Debugf("PublishWithEOL %v to %s", id, repubkey)
	return ns.dstore.Put(dshelp.NewKeyFromBinary([]byte(repubkey)), rec)
}

// RepubKeyForID gets the key in the datastore at which a peer's record will be stored
func RepubKeyForID(id peer.ID) string {
	return "/repub/" + string(id)
}
