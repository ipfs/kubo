package integrationtest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	mock "github.com/ipfs/go-ipfs/core/mock"
	"github.com/ipfs/go-ipfs/thirdparty/unit"

	pstore "gx/ipfs/QmQAGG1zxfePqj2t7bLxyN8AFccZ889DDR9Gn8kVLDrGZo/go-libp2p-peerstore"
	mocknet "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/net/mock"
	testutil "gx/ipfs/QmZXjR5X1p4KrQ967cTsy4MymMzUM8mZECF3PV8UcN4o3g/go-testutil"
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
	mn := mocknet.New(ctx)
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

	catterApi := coreapi.NewCoreAPI(catter)

	err = mn.LinkAll()
	if err != nil {
		return err
	}

	bs1 := []pstore.PeerInfo{adder.Peerstore.PeerInfo(adder.Identity)}
	bs2 := []pstore.PeerInfo{catter.Peerstore.PeerInfo(catter.Identity)}

	if err := catter.Bootstrap(core.BootstrapConfigWithPeers(bs1)); err != nil {
		return err
	}
	if err := adder.Bootstrap(core.BootstrapConfigWithPeers(bs2)); err != nil {
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

	b.StartTimer()
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
	return nil
}
