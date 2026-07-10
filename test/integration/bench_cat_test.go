package integrationtest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/ipfs/boxo/bootstrap"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	mock "github.com/ipfs/kubo/core/mock"
	"github.com/ipfs/kubo/thirdparty/unit"
	testutil "github.com/libp2p/go-libp2p-testing/net"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

func BenchmarkCat1MB(b *testing.B) { benchmarkVarCat(b, unit.MB*1) }
func BenchmarkCat2MB(b *testing.B) { benchmarkVarCat(b, unit.MB*2) }
func BenchmarkCat4MB(b *testing.B) { benchmarkVarCat(b, unit.MB*4) }

func benchmarkVarCat(b *testing.B, size int64) {
	data := RandomBytes(size)
	b.SetBytes(size)
	for n := 0; n < b.N; n++ {
		err := benchCat(b, data, instant)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchCat(b *testing.B, data []byte, conf testutil.LatencyConfig) error {
	b.StopTimer()
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

	b.StartTimer()
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
