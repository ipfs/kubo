package dht

import (
	"context"
	"fmt"
	"time"

	ctxfrac "gx/ipfs/QmTKsRYeY4simJyf37K93juSq75Lo8MVCDJ7owjmf46u8W/go-context/frac"
	routing "gx/ipfs/QmbkGVaN9W6RYJK4Ws5FvMKXKDqdRQ5snhtaa92qP6L8eU/go-libp2p-routing"
	record "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record"
	recpb "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record/pb"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	ci "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
)

// MaxRecordAge specifies the maximum time that any node will hold onto a record
// from the time its received. This does not apply to any other forms of validity that
// the record may contain.
// For example, a record may contain an ipns entry with an EOL saying its valid
// until the year 2020 (a great time in the future). For that record to stick around
// it must be rebroadcasted more frequently than once every 'MaxRecordAge'
const MaxRecordAge = time.Hour * 24 * 7

func (dht *IpfsDHT) GetPublicKey(ctx context.Context, p peer.ID) (ci.PubKey, error) {
	log.Debugf("getPublicKey for: %s", p)

	// check locally.
	pk := dht.peerstore.PubKey(p)
	if pk != nil {
		return pk, nil
	}

	// ok, try the node itself. if they're overwhelmed or slow we can move on.
	ctxT, cancelFunc := ctxfrac.WithDeadlineFraction(ctx, 0.3)
	defer cancelFunc()
	if pk, err := dht.getPublicKeyFromNode(ctx, p); err == nil {
		err := dht.peerstore.AddPubKey(p, pk)
		if err != nil {
			return pk, err
		}
		return pk, nil
	}

	// last ditch effort: let's try the dht.
	log.Debugf("pk for %s not in peerstore, and peer failed. Trying DHT.", p)
	pkkey := routing.KeyForPublicKey(p)

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

	return pk, dht.peerstore.AddPubKey(p, pk)
}

func (dht *IpfsDHT) getPublicKeyFromNode(ctx context.Context, p peer.ID) (ci.PubKey, error) {

	// check locally, just in case...
	pk := dht.peerstore.PubKey(p)
	if pk != nil {
		return pk, nil
	}

	pkkey := routing.KeyForPublicKey(p)
	pmes, err := dht.getValueSingle(ctx, p, pkkey)
	if err != nil {
		return nil, err
	}

	// node doesn't have key :(
	record := pmes.GetRecord()
	if record == nil {
		return nil, fmt.Errorf("Node not responding with its public key: %s", p)
	}

	// Success! We were given the value. we don't need to check
	// validity because a) we can't. b) we know the hash of the
	// key we're looking for.
	val := record.GetValue()
	log.Debug("DHT got a value from other peer")

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
	log.Debugf("DHT got public key from node itself.")
	return pk, nil
}

// verifyRecordLocally attempts to verify a record. if we do not have the public
// key, we fail. we do not search the dht.
func (dht *IpfsDHT) verifyRecordLocally(r *recpb.Record) error {
	if r == nil {
		log.Error("nil record passed into verifyRecordLocally")
		return fmt.Errorf("nil record")
	}

	if len(r.Signature) > 0 {
		// First, validate the signature
		p := peer.ID(r.GetAuthor())
		pk := dht.peerstore.PubKey(p)
		if pk == nil {
			return fmt.Errorf("do not have public key for %s", p)
		}

		if err := record.CheckRecordSig(r, pk); err != nil {
			return err
		}
	}

	return dht.Validator.VerifyRecord(r)
}

// verifyRecordOnline verifies a record, searching the DHT for the public key
// if necessary. The reason there is a distinction in the functions is that
// retrieving arbitrary public keys from the DHT as a result of passively
// receiving records (e.g. through a PUT_VALUE or ADD_PROVIDER) can cause a
// massive amplification attack on the dht. Use with care.
func (dht *IpfsDHT) verifyRecordOnline(ctx context.Context, r *recpb.Record) error {

	if len(r.Signature) > 0 {
		// get the public key, search for it if necessary.
		p := peer.ID(r.GetAuthor())
		pk, err := dht.GetPublicKey(ctx, p)
		if err != nil {
			return err
		}

		err = record.CheckRecordSig(r, pk)
		if err != nil {
			return err
		}
	}

	return dht.Validator.VerifyRecord(r)
}
