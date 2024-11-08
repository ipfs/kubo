package integrationtest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"testing"
	"time"

	"github.com/ipfs/boxo/bootstrap"
	"github.com/ipfs/boxo/files"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	mock "github.com/ipfs/kubo/core/mock"
	"github.com/ipfs/kubo/thirdparty/unit"
	testutil "github.com/libp2p/go-libp2p-testing/net"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

var log = logging.Logger("epictest")

const kSeed = 1

func Test1KBInstantaneous(t *testing.T) {
	conf := testutil.LatencyConfig{
		NetworkLatency:    0,
		RoutingLatency:    0,
		BlockstoreLatency: 0,
	}

	if err := DirectAddCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowBlockstore(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{BlockstoreLatency: 50 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowNetwork(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{NetworkLatency: 400 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowRouting(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{RoutingLatency: 400 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func Test100MBMacbookCoastToCoast(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{}.NetworkNYtoSF().BlockstoreSlowSSD2014().RoutingSlow()
	if err := DirectAddCat(RandomBytes(100*1024*1024), conf); err != nil {
		t.Fatal(err)
	}
}

func AddCatPowers(conf testutil.LatencyConfig, megabytesMax int64) error {
	var i int64
	for i = 1; i < megabytesMax; i = i * 2 {
		fmt.Printf("%d MB\n", i)
		if err := DirectAddCat(RandomBytes(i*1024*1024), conf); err != nil {
			return err
		}
	}
	return nil
}

func RandomBytes(n int64) []byte {
	random.SetSeed(kSeed)
	return random.Bytes(int(n))
}

func DirectAddCat(data []byte, conf testutil.LatencyConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create network
	mn := mocknet.New()
	mn.SetLinkDefaults(mocknet.LinkOptions{
		Latency: conf.NetworkLatency,
		// TODO add to conf. This is tricky because we want 0 values to be functional.
		Bandwidth: math.MaxInt32,
	})

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

	adderAPI, err := coreapi.NewCoreAPI(adder)
	if err != nil {
		return err
	}

	catterAPI, err := coreapi.NewCoreAPI(catter)
	if err != nil {
		return err
	}

	err = mn.LinkAll()
	if err != nil {
		return err
	}

	bs1 := []peer.AddrInfo{adder.Peerstore.PeerInfo(adder.Identity)}
	bs2 := []peer.AddrInfo{catter.Peerstore.PeerInfo(catter.Identity)}

	if err := catter.Bootstrap(bootstrap.BootstrapConfigWithPeers(bs1)); err != nil {
		return err
	}
	if err := adder.Bootstrap(bootstrap.BootstrapConfigWithPeers(bs2)); err != nil {
		return err
	}

	added, err := adderAPI.Unixfs().Add(ctx, files.NewBytesFile(data))
	if err != nil {
		return err
	}

	readerCatted, err := catterAPI.Unixfs().Get(ctx, added)
	if err != nil {
		return err
	}

	// verify
	var bufout bytes.Buffer
	_, err = io.Copy(&bufout, readerCatted.(io.Reader))
	if err != nil {
		return err
	}
	if !bytes.Equal(bufout.Bytes(), data) {
		return errors.New("catted data does not match added data")
	}

	return nil
}

func SkipUnlessEpic(t *testing.T) {
	if os.Getenv("IPFS_EPIC_TEST") == "" {
		t.SkipNow()
	}
}
