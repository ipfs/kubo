package integrationtest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	mock "github.com/ipfs/go-ipfs/core/mock"
	"github.com/ipfs/go-ipfs/thirdparty/unit"

	pstore "gx/ipfs/QmQAGG1zxfePqj2t7bLxyN8AFccZ889DDR9Gn8kVLDrGZo/go-libp2p-peerstore"
	mocknet "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/net/mock"
	testutil "gx/ipfs/QmZXjR5X1p4KrQ967cTsy4MymMzUM8mZECF3PV8UcN4o3g/go-testutil"
)

func TestThreeLeggedCatTransfer(t *testing.T) {
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
	conf := testutil.LatencyConfig{}.NetworkNYtoSF().BlockstoreSlowSSD2014().RoutingSlow()
	if err := RunThreeLeggedCat(RandomBytes(100*unit.MB), conf); err != nil {
		t.Fatal(err)
	}
}

func RunThreeLeggedCat(data []byte, conf testutil.LatencyConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create network
	mn := mocknet.New(ctx)
	mn.SetLinkDefaults(mocknet.LinkOptions{
		Latency: conf.NetworkLatency,
		// TODO add to conf. This is tricky because we want 0 values to be functional.
		Bandwidth: math.MaxInt32,
	})

	bootstrap, err := core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		Host:   mock.MockHostOption(mn),
	})
	if err != nil {
		return err
	}
	defer bootstrap.Close()

	adder, err := core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		Host:   mock.MockHostOption(mn),
	})
	if err != nil {
		return err
	}
	defer adder.Close()

	catter, err := core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		Host:   mock.MockHostOption(mn),
	})
	if err != nil {
		return err
	}
	defer catter.Close()

	catterApi := coreapi.NewCoreAPI(catter)

	mn.LinkAll()

	bis := bootstrap.Peerstore.PeerInfo(bootstrap.PeerHost.ID())
	bcfg := core.BootstrapConfigWithPeers([]pstore.PeerInfo{bis})
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

	ap, err := iface.ParsePath(added)
	if err != nil {
		return err
	}

	readerCatted, err := catterApi.Unixfs().Get(ctx, ap)
	if err != nil {
		return err
	}

	// verify
	bufout := new(bytes.Buffer)
	io.Copy(bufout, readerCatted)
	if 0 != bytes.Compare(bufout.Bytes(), data) {
		return errors.New("catted data does not match added data")
	}
	cancel()
	return nil
}
