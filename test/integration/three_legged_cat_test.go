package integrationtest

import (
	"bytes"
	"io"
	"math"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	core "github.com/jbenet/go-ipfs/core"
	coreunix "github.com/jbenet/go-ipfs/core/coreunix"
	mocknet "github.com/jbenet/go-ipfs/p2p/net/mock"
	"github.com/jbenet/go-ipfs/p2p/peer"
	"github.com/jbenet/go-ipfs/thirdparty/unit"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestThreeLeggedCat1KBInstantaneous(t *testing.T) {
	conf := testutil.LatencyConfig{
		NetworkLatency:    0,
		RoutingLatency:    0,
		BlockstoreLatency: 0,
	}
	if err := RunThreeLeggedCat(RandomBytes(100*unit.MB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestThreeLeggedCatDegenerateSlowBlockstore(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{BlockstoreLatency: 50 * time.Millisecond}
	if err := RunThreeLeggedCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestThreeLeggedCatDegenerateSlowNetwork(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{NetworkLatency: 400 * time.Millisecond}
	if err := RunThreeLeggedCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestThreeLeggedCatDegenerateSlowRouting(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{RoutingLatency: 400 * time.Millisecond}
	if err := RunThreeLeggedCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestThreeLeggedCat100MBMacbookCoastToCoast(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{}.Network_NYtoSF().Blockstore_SlowSSD2014().Routing_Slow()
	if err := RunThreeLeggedCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func RunThreeLeggedCat(data []byte, conf testutil.LatencyConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const numPeers = 3

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

	peers := mn.Peers()
	if len(peers) < numPeers {
		return errors.New("test initialization error")
	}
	bootstrap, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[2], mn.Host(peers[2]), conf))
	if err != nil {
		return err
	}
	defer bootstrap.Close()
	adder, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[0], mn.Host(peers[0]), conf))
	if err != nil {
		return err
	}
	defer adder.Close()
	catter, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[1], mn.Host(peers[1]), conf))
	if err != nil {
		return err
	}
	defer catter.Close()

	bis := bootstrap.Peerstore.PeerInfo(bootstrap.PeerHost.ID())
	bcfg := core.BootstrapConfigWithPeers([]peer.PeerInfo{bis})
	if err := adder.Bootstrap(bcfg); err != nil {
		return err
	}
	if err := catter.Bootstrap(bcfg); err != nil {
		return err
	}

	added, err := coreunix.Add(adder, bytes.NewReader(data))
	if err != nil {
		return err
	}

	readerCatted, err := coreunix.Cat(catter, added)
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
