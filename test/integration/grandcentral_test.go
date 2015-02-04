package integrationtest

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/core/corerouting"
	"github.com/jbenet/go-ipfs/core/coreunix"
	mocknet "github.com/jbenet/go-ipfs/p2p/net/mock"
	"github.com/jbenet/go-ipfs/p2p/peer"
	"github.com/jbenet/go-ipfs/thirdparty/iter"
	"github.com/jbenet/go-ipfs/thirdparty/unit"
	"github.com/jbenet/go-ipfs/util"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestSupernodeBootstrappedAddCat(t *testing.T) {
	// create 8 supernode-routing bootstrap nodes
	// create 2 supernode-routing clients both bootstrapped to the bootstrap nodes
	// let the bootstrap nodes share a single datastore
	// add a large file on one node then cat the file from the other
	conf := testutil.LatencyConfig{
		NetworkLatency:    0,
		RoutingLatency:    0,
		BlockstoreLatency: 0,
	}
	if err := RunSupernodeBootstrappedAddCat(RandomBytes(100*unit.MB), conf); err != nil {
		t.Fatal(err)
	}
}

func RunSupernodeBootstrappedAddCat(data []byte, conf testutil.LatencyConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	servers, clients, err := InitializeSupernodeNetwork(ctx, 8, 2, conf)
	if err != nil {
		return err
	}
	for _, n := range append(servers, clients...) {
		defer n.Close()
	}

	adder := clients[0]
	catter := clients[1]

	log.Critical("adder is", adder.Identity)
	log.Critical("catter is", catter.Identity)

	keyAdded, err := coreunix.Add(adder, bytes.NewReader(data))
	if err != nil {
		return err
	}

	readerCatted, err := coreunix.Cat(catter, keyAdded)
	if err != nil {
		return err
	}

	// verify
	var bufout bytes.Buffer
	io.Copy(&bufout, readerCatted)
	if 0 != bytes.Compare(bufout.Bytes(), data) {
		return errors.New("catted data does not match added data")
	}
	return nil
}

func InitializeSupernodeNetwork(
	ctx context.Context,
	numServers, numClients int,
	conf testutil.LatencyConfig) ([]*core.IpfsNode, []*core.IpfsNode, error) {

	// create network
	mn, err := mocknet.FullMeshLinked(ctx, numServers+numClients)
	if err != nil {
		return nil, nil, errors.Wrap(err)
	}

	mn.SetLinkDefaults(mocknet.LinkOptions{
		Latency:   conf.NetworkLatency,
		Bandwidth: math.MaxInt32,
	})

	peers := mn.Peers()
	if len(peers) < numServers+numClients {
		return nil, nil, errors.New("test initialization error")
	}
	clientPeers, serverPeers := peers[0:numClients], peers[numClients:]

	routingDatastore := ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore()))
	var servers []*core.IpfsNode
	for i := range iter.N(numServers) {
		p := serverPeers[i]
		bootstrap, err := core.NewIPFSNode(ctx, MocknetTestRepo(p, mn.Host(p), conf,
			corerouting.SupernodeServer(routingDatastore)))
		if err != nil {
			return nil, nil, err
		}
		servers = append(servers, bootstrap)
	}

	var bootstrapInfos []peer.PeerInfo
	for _, n := range servers {
		info := n.Peerstore.PeerInfo(n.PeerHost.ID())
		bootstrapInfos = append(bootstrapInfos, info)
	}

	var clients []*core.IpfsNode
	for i := range iter.N(numClients) {
		p := clientPeers[i]
		n, err := core.NewIPFSNode(ctx, MocknetTestRepo(p, mn.Host(p), conf,
			corerouting.SupernodeClient(bootstrapInfos...)))
		if err != nil {
			return nil, nil, err
		}
		clients = append(clients, n)
	}

	bcfg := core.BootstrapConfigWithPeers(bootstrapInfos)
	for _, n := range clients {
		if err := n.Bootstrap(bcfg); err != nil {
			return nil, nil, err
		}
	}
	return servers, clients, nil
}

func TestSupernodePutRecordGetRecord(t *testing.T) {
	// create 8 supernode-routing bootstrap nodes
	// create 2 supernode-routing clients both bootstrapped to the bootstrap nodes
	// let the bootstrap nodes share a single datastore
	// add a large file on one node then cat the file from the other
	conf := testutil.LatencyConfig{
		NetworkLatency:    0,
		RoutingLatency:    0,
		BlockstoreLatency: 0,
	}
	if err := RunSupernodePutRecordGetRecord(conf); err != nil {
		t.Fatal(err)
	}
}

func RunSupernodePutRecordGetRecord(conf testutil.LatencyConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	servers, clients, err := InitializeSupernodeNetwork(ctx, 2, 2, conf)
	if err != nil {
		return err
	}
	for _, n := range append(servers, clients...) {
		defer n.Close()
	}

	putter := clients[0]
	getter := clients[1]

	k := util.Key("key")
	note := []byte("a note from putter")

	if err := putter.Routing.PutValue(ctx, k, note); err != nil {
		return fmt.Errorf("failed to put value: %s", err)
	}

	received, err := getter.Routing.GetValue(ctx, k)
	if err != nil {
		return fmt.Errorf("failed to get value: %s", err)
	}

	if 0 != bytes.Compare(note, received) {
		return errors.New("record doesn't match")
	}
	return nil
}
