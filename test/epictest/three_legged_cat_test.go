package epictest

import (
	"bytes"
	"io"
	"math"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	mocknet "github.com/jbenet/go-ipfs/p2p/net/mock"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestThreeLeggedCat(t *testing.T) {
	conf := testutil.LatencyConfig{
		NetworkLatency:    0,
		RoutingLatency:    0,
		BlockstoreLatency: 0,
	}
	if err := RunThreeLeggedCat(RandomBytes(1*KB), conf); err != nil {
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
	adder, err := makeCore(ctx, MocknetTestRepo(peers[0], mn.Host(peers[0]), conf))
	if err != nil {
		return err
	}
	catter, err := makeCore(ctx, MocknetTestRepo(peers[1], mn.Host(peers[1]), conf))
	if err != nil {
		return err
	}
	bootstrap, err := makeCore(ctx, MocknetTestRepo(peers[2], mn.Host(peers[2]), conf))
	if err != nil {
		return err
	}

	adder.Bootstrap(ctx, bootstrap.ID())
	catter.Bootstrap(ctx, bootstrap.ID())

	keyAdded, err := adder.Add(bytes.NewReader(data))
	if err != nil {
		return err
	}

	readerCatted, err := catter.Cat(keyAdded)
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
