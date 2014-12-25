package epictest

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	random "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-random"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	blockservice "github.com/jbenet/go-ipfs/blockservice"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	importer "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	net "github.com/jbenet/go-ipfs/net"
	mocknet "github.com/jbenet/go-ipfs/net/mock"
	path "github.com/jbenet/go-ipfs/path"
	peer "github.com/jbenet/go-ipfs/peer"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	util "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/datastore2"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

const kSeed = 1

func Test1KBInstantaneous(t *testing.T) {
	conf := Config{
		NetworkLatency:    0,
		RoutingLatency:    0,
		BlockstoreLatency: 0,
	}

	if err := AddCatBytes(RandomBytes(100*MB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowBlockstore(t *testing.T) {
	SkipUnlessEpic(t)
	conf := Config{BlockstoreLatency: 50 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowNetwork(t *testing.T) {
	SkipUnlessEpic(t)
	conf := Config{NetworkLatency: 400 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowRouting(t *testing.T) {
	SkipUnlessEpic(t)
	conf := Config{RoutingLatency: 400 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func Test100MBMacbookCoastToCoast(t *testing.T) {
	SkipUnlessEpic(t)
	conf := Config{}.Network_NYtoSF().Blockstore_SlowSSD2014().Routing_Slow()
	if err := AddCatBytes(RandomBytes(100*1024*1024), conf); err != nil {
		t.Fatal(err)
	}
}

func AddCatPowers(conf Config, megabytesMax int64) error {
	var i int64
	for i = 1; i < megabytesMax; i = i * 2 {
		fmt.Printf("%d MB\n", i)
		if err := AddCatBytes(RandomBytes(i*1024*1024), conf); err != nil {
			return err
		}
	}
	return nil
}

func RandomBytes(n int64) []byte {
	var data bytes.Buffer
	random.WritePseudoRandomBytes(n, &data, kSeed)
	return data.Bytes()
}

type instance struct {
	ID             peer.ID
	Network        net.Network
	Blockstore     blockstore.Blockstore
	Datastore      datastore.ThreadSafeDatastore
	DHT            *dht.IpfsDHT
	Exchange       exchange.Interface
	BitSwapNetwork bsnet.BitSwapNetwork

	datastoreDelay delay.D
}

func AddCatBytes(data []byte, conf Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const numPeers = 2
	instances := make(map[peer.ID]*instance, numPeers)

	// create network
	mn, err := mocknet.FullMeshLinked(ctx, numPeers)
	if err != nil {
		return errors.Wrap(err)
	}
	mn.SetLinkDefaults(mocknet.LinkOptions{
		Latency: conf.NetworkLatency,
		// TODO add to conf. This is tricky because we want 0 values to be functional.
		Bandwidth: math.MaxInt32,
	})
	for _, p := range mn.Peers() {
		instances[p] = &instance{
			ID:      p,
			Network: mn.Net(p),
		}
	}

	// create dht network
	for _, p := range mn.Peers() {
		dsDelay := delay.Fixed(conf.BlockstoreLatency)
		instances[p].Datastore = sync.MutexWrap(datastore2.WithDelay(datastore.NewMapDatastore(), dsDelay))
		instances[p].datastoreDelay = dsDelay
	}
	for _, p := range mn.Peers() {
		instances[p].DHT = dht.NewDHT(ctx, p, instances[p].Network, instances[p].Datastore)
	}
	// create two bitswap network clients
	for _, p := range mn.Peers() {
		instances[p].BitSwapNetwork = bsnet.NewFromIpfsNetwork(instances[p].Network, instances[p].DHT)
	}
	for _, p := range mn.Peers() {
		const kWriteCacheElems = 100
		const alwaysSendToPeer = true
		adapter := instances[p].BitSwapNetwork
		dstore := instances[p].Datastore
		instances[p].Blockstore, err = blockstore.WriteCached(blockstore.NewBlockstore(dstore), kWriteCacheElems)
		if err != nil {
			return err
		}
		instances[p].Exchange = bitswap.New(ctx, p, adapter, instances[p].Blockstore, alwaysSendToPeer)
	}
	var peers []peer.ID
	for _, p := range mn.Peers() {
		peers = append(peers, p)
	}

	adder := instances[peers[0]]
	catter := instances[peers[1]]

	// bootstrap the DHTs
	adder.DHT.Connect(ctx, catter.ID)
	catter.DHT.Connect(ctx, adder.ID)

	adder.datastoreDelay.Set(0) // disable blockstore latency during add operation
	keyAdded, err := add(adder, bytes.NewReader(data))
	if err != nil {
		return err
	}
	adder.datastoreDelay.Set(conf.BlockstoreLatency) // add some blockstore delay to make the catter wait

	readerCatted, err := cat(catter, keyAdded)
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

func cat(catter *instance, k util.Key) (io.Reader, error) {
	catterdag := merkledag.NewDAGService(&blockservice.BlockService{catter.Blockstore, catter.Exchange})
	nodeCatted, err := (&path.Resolver{catterdag}).ResolvePath(k.String())
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(nodeCatted, catterdag)
}

func add(adder *instance, r io.Reader) (util.Key, error) {
	nodeAdded, err := importer.BuildDagFromReader(
		r,
		merkledag.NewDAGService(&blockservice.BlockService{adder.Blockstore, adder.Exchange}),
		nil,
		chunk.DefaultSplitter,
	)
	if err != nil {
		return "", err
	}
	return nodeAdded.Key()
}

func SkipUnlessEpic(t *testing.T) {
	if os.Getenv("IPFS_EPIC_TEST") == "" {
		t.SkipNow()
	}
}
