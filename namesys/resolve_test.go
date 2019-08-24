package namesys

import (
	"context"
	"errors"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	mockrouting "github.com/ipfs/go-ipfs-routing/mock"
	ipns "github.com/ipfs/go-ipns"
	path "github.com/ipfs/go-path"
	testutil "github.com/libp2p/go-libp2p-testing/net"
	tnet "github.com/libp2p/go-libp2p-testing/net"
)

func TestRoutingResolve(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	serv := mockrouting.NewServer()
	id := testutil.RandIdentityOrFatal(t)
	d := serv.ClientWithDatastore(context.Background(), id, dstore)

	resolver := NewIpnsResolver(d)
	publisher := NewIpnsPublisher(d, dstore)

	identity := tnet.RandIdentityOrFatal(t)

	h := path.FromString("/ipfs/QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN")
	err := publisher.Publish(context.Background(), identity.PrivateKey(), h)
	if err != nil {
		t.Fatal(err)
	}

	res, err := resolver.Resolve(context.Background(), identity.ID().Pretty())
	if err != nil {
		t.Fatal(err)
	}

	if res != h {
		t.Fatal("Got back incorrect value.")
	}
}

func TestPrexistingExpiredRecord(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	d := mockrouting.NewServer().ClientWithDatastore(context.Background(), testutil.RandIdentityOrFatal(t), dstore)

	resolver := NewIpnsResolver(d)
	publisher := NewIpnsPublisher(d, dstore)

	identity := tnet.RandIdentityOrFatal(t)

	// Make an expired record and put it in the datastore
	h := path.FromString("/ipfs/QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN")
	eol := time.Now().Add(time.Hour * -1)

	entry, err := ipns.Create(identity.PrivateKey(), []byte(h), 0, eol)
	if err != nil {
		t.Fatal(err)
	}
	err = PutRecordToRouting(context.Background(), d, identity.PublicKey(), entry)
	if err != nil {
		t.Fatal(err)
	}

	// Now, with an old record in the system already, try and publish a new one
	err = publisher.Publish(context.Background(), identity.PrivateKey(), h)
	if err != nil {
		t.Fatal(err)
	}

	err = verifyCanResolve(resolver, identity.ID().Pretty(), h)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrexistingRecord(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	d := mockrouting.NewServer().ClientWithDatastore(context.Background(), testutil.RandIdentityOrFatal(t), dstore)

	resolver := NewIpnsResolver(d)
	publisher := NewIpnsPublisher(d, dstore)

	identity := tnet.RandIdentityOrFatal(t)

	// Make a good record and put it in the datastore
	h := path.FromString("/ipfs/QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN")
	eol := time.Now().Add(time.Hour)
	entry, err := ipns.Create(identity.PrivateKey(), []byte(h), 0, eol)
	if err != nil {
		t.Fatal(err)
	}
	err = PutRecordToRouting(context.Background(), d, identity.PublicKey(), entry)
	if err != nil {
		t.Fatal(err)
	}

	// Now, with an old record in the system already, try and publish a new one
	err = publisher.Publish(context.Background(), identity.PrivateKey(), h)
	if err != nil {
		t.Fatal(err)
	}

	err = verifyCanResolve(resolver, identity.ID().Pretty(), h)
	if err != nil {
		t.Fatal(err)
	}
}

func verifyCanResolve(r Resolver, name string, exp path.Path) error {
	res, err := r.Resolve(context.Background(), name)
	if err != nil {
		return err
	}

	if res != exp {
		return errors.New("got back wrong record")
	}

	return nil
}
