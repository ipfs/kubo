package dht

import (
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	u "github.com/jbenet/go-ipfs/util"
	ctxutil "github.com/jbenet/go-ipfs/util/ctx"
)

// KeyForPublicKey returns the key used to retrieve public keys
// from the dht.
func KeyForPublicKey(id peer.ID) u.Key {
	return u.Key("/pk/" + string(id))
}

func (dht *IpfsDHT) getPublicKeyOnline(ctx context.Context, p peer.ID) (ci.PubKey, error) {
	log.Debugf("getPublicKey for: %s", p)

	// check locally.
	pk := dht.peerstore.PubKey(p)
	if pk != nil {
		return pk, nil
	}

	// ok, try the node itself. if they're overwhelmed or slow we can move on.
	ctxT, _ := ctxutil.WithDeadlineFraction(ctx, 0.3)
	if pk, err := dht.getPublicKeyFromNode(ctx, p); err == nil {
		return pk, nil
	}

	// last ditch effort: let's try the dht.
	log.Debugf("pk for %s not in peerstore, and peer failed. trying dht.", p)
	pkkey := KeyForPublicKey(p)

	// ok, try the node itself. if they're overwhelmed or slow we can move on.
	val, err := dht.GetValue(ctxT, pkkey)
	if err != nil {
		log.Warning("Failed to find requested public key.")
		return nil, err
	}

	pk, err = ci.UnmarshalPublicKey(val)
	if err != nil {
		log.Debugf("Failed to unmarshal public key: %s", err)
		return nil, err
	}
	return pk, nil
}

func (dht *IpfsDHT) getPublicKeyFromNode(ctx context.Context, p peer.ID) (ci.PubKey, error) {

	// check locally, just in case...
	pk := dht.peerstore.PubKey(p)
	if pk != nil {
		return pk, nil
	}

	pkkey := KeyForPublicKey(p)
	pmes, err := dht.getValueSingle(ctx, p, pkkey)
	if err != nil {
		return nil, err
	}

	// node doesn't have key :(
	record := pmes.GetRecord()
	if record == nil {
		return nil, fmt.Errorf("node not responding with its public key: %s", p)
	}

	// Success! We were given the value. we don't need to check
	// validity because a) we can't. b) we know the hash of the
	// key we're looking for.
	val := record.GetValue()
	log.Debug("dht got a value from other peer.")

	pk, err = ci.UnmarshalPublicKey(val)
	if err != nil {
		return nil, err
	}

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return nil, err
	}
	if id != p {
		return nil, fmt.Errorf("public key does not match id: %s", p)
	}

	// ok! it's valid. we got it!
	log.Debugf("dht got public key from node itself.")
	return pk, nil
}

// verifyRecordLocally attempts to verify a record. if we do not have the public
// key, we fail. we do not search the dht.
func (dht *IpfsDHT) verifyRecordLocally(r *pb.Record) error {

	// First, validate the signature
	p := peer.ID(r.GetAuthor())
	pk := dht.peerstore.PubKey(p)
	if pk == nil {
		return fmt.Errorf("do not have public key for %s", p)
	}

	return dht.Validator.VerifyRecord(r, pk)
}

// verifyRecordOnline verifies a record, searching the DHT for the public key
// if necessary. The reason there is a distinction in the functions is that
// retrieving arbitrary public keys from the DHT as a result of passively
// receiving records (e.g. through a PUT_VALUE or ADD_PROVIDER) can cause a
// massive amplification attack on the dht. Use with care.
func (dht *IpfsDHT) verifyRecordOnline(ctx context.Context, r *pb.Record) error {

	// get the public key, search for it if necessary.
	p := peer.ID(r.GetAuthor())
	pk, err := dht.getPublicKeyOnline(ctx, p)
	if err != nil {
		return err
	}

	return dht.Validator.VerifyRecord(r, pk)
}
