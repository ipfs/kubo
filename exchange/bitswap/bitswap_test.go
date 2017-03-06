package bitswap

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	blocks "github.com/ipfs/go-ipfs/blocks"
	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	blocksutil "github.com/ipfs/go-ipfs/blocks/blocksutil"
	tn "github.com/ipfs/go-ipfs/exchange/bitswap/testnet"
	mockrouting "github.com/ipfs/go-ipfs/routing/mock"
	delay "github.com/ipfs/go-ipfs/thirdparty/delay"
	travis "github.com/ipfs/go-ipfs/thirdparty/testutil/ci/travis"

	detectrace "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-detect-race"

	p2ptestutil "gx/ipfs/QmNqvnxGtJBaKQnenD6uboNGdjSjHGmZGRxMHEevKJe5Pk/go-libp2p-netutil"
	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
)

// FIXME the tests are really sensitive to the network delay. fix them to work
// well under varying conditions
const kNetworkDelay = 0 * time.Millisecond

func getVirtualNetwork() tn.Network {
	return tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
}

func TestClose(t *testing.T) {
	vnet := getVirtualNetwork()
	sesgen := NewTestSessionGenerator(vnet)
	defer sesgen.Close()
	bgen := blocksutil.NewBlockGenerator()

	block := bgen.Next()
	bitswap := sesgen.Next()

	bitswap.Exchange.Close()
	bitswap.Exchange.GetBlock(context.Background(), block.Cid())
}

func TestProviderForKeyButNetworkCannotFind(t *testing.T) { // TODO revisit this

	rs := mockrouting.NewServer()
	net := tn.VirtualNetwork(rs, delay.Fixed(kNetworkDelay))
	g := NewTestSessionGenerator(net)
	defer g.Close()

	block := blocks.NewBlock([]byte("block"))
	pinfo := p2ptestutil.RandTestBogusIdentityOrFatal(t)
	rs.Client(pinfo).Provide(context.Background(), block.Cid()) // but not on network

	solo := g.Next()
	defer solo.Exchange.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	_, err := solo.Exchange.GetBlock(ctx, block.Cid())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

func TestGetBlockFromPeerAfterPeerAnnounces(t *testing.T) {

	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
	block := blocks.NewBlock([]byte("block"))
	g := NewTestSessionGenerator(net)
	defer g.Close()

	peers := g.Instances(2)
	hasBlock := peers[0]
	defer hasBlock.Exchange.Close()

	if err := hasBlock.Exchange.HasBlock(block); err != nil {
		t.Fatal(err)
	}

	wantsBlock := peers[1]
	defer wantsBlock.Exchange.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	received, err := wantsBlock.Exchange.GetBlock(ctx, block.Cid())
	if err != nil {
		t.Log(err)
		t.Fatal("Expected to succeed")
	}

	if !bytes.Equal(block.RawData(), received.RawData()) {
		t.Fatal("Data doesn't match")
	}
}

func TestLargeSwarm(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	numInstances := 100
	numBlocks := 2
	if detectrace.WithRace() {
		// when running with the race detector, 500 instances launches
		// well over 8k goroutines. This hits a race detector limit.
		numInstances = 100
	} else if travis.IsRunning() {
		numInstances = 200
	} else {
		t.Parallel()
	}
	PerformDistributionTest(t, numInstances, numBlocks)
}

func TestLargeFile(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	if !travis.IsRunning() {
		t.Parallel()
	}

	numInstances := 10
	numBlocks := 100
	PerformDistributionTest(t, numInstances, numBlocks)
}

func TestLargeFileNoRebroadcast(t *testing.T) {
	rbd := rebroadcastDelay.Get()
	rebroadcastDelay.Set(time.Hour * 24 * 365 * 10) // ten years should be long enough
	if testing.Short() {
		t.SkipNow()
	}
	numInstances := 10
	numBlocks := 100
	PerformDistributionTest(t, numInstances, numBlocks)
	rebroadcastDelay.Set(rbd)
}

func TestLargeFileTwoPeers(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	numInstances := 2
	numBlocks := 100
	PerformDistributionTest(t, numInstances, numBlocks)
}

func PerformDistributionTest(t *testing.T, numInstances, numBlocks int) {
	ctx := context.Background()
	if testing.Short() {
		t.SkipNow()
	}
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
	sg := NewTestSessionGenerator(net)
	defer sg.Close()
	bg := blocksutil.NewBlockGenerator()

	instances := sg.Instances(numInstances)
	blocks := bg.Blocks(numBlocks)

	t.Log("Give the blocks to the first instance")

	nump := len(instances) - 1
	// assert we're properly connected
	for _, inst := range instances {
		peers := inst.Exchange.wm.ConnectedPeers()
		for i := 0; i < 10 && len(peers) != nump; i++ {
			time.Sleep(time.Millisecond * 50)
			peers = inst.Exchange.wm.ConnectedPeers()
		}
		if len(peers) != nump {
			t.Fatal("not enough peers connected to instance")
		}
	}

	var blkeys []*cid.Cid
	first := instances[0]
	for _, b := range blocks {
		blkeys = append(blkeys, b.Cid())
		first.Exchange.HasBlock(b)
	}

	t.Log("Distribute!")

	wg := sync.WaitGroup{}
	errs := make(chan error)

	for _, inst := range instances[1:] {
		wg.Add(1)
		go func(inst Instance) {
			defer wg.Done()
			outch, err := inst.Exchange.GetBlocks(ctx, blkeys)
			if err != nil {
				errs <- err
			}
			for _ = range outch {
			}
		}(inst)
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Log("Verify!")

	for _, inst := range instances {
		for _, b := range blocks {
			if _, err := inst.Blockstore().Get(b.Cid()); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func getOrFail(bitswap Instance, b blocks.Block, t *testing.T, wg *sync.WaitGroup) {
	if _, err := bitswap.Blockstore().Get(b.Cid()); err != nil {
		_, err := bitswap.Exchange.GetBlock(context.Background(), b.Cid())
		if err != nil {
			t.Fatal(err)
		}
	}
	wg.Done()
}

// TODO simplify this test. get to the _essence_!
func TestSendToWantingPeer(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
	sg := NewTestSessionGenerator(net)
	defer sg.Close()
	bg := blocksutil.NewBlockGenerator()

	prev := rebroadcastDelay.Set(time.Second / 2)
	defer func() { rebroadcastDelay.Set(prev) }()

	peers := sg.Instances(2)
	peerA := peers[0]
	peerB := peers[1]

	t.Logf("Session %v\n", peerA.Peer)
	t.Logf("Session %v\n", peerB.Peer)

	waitTime := time.Second * 5

	alpha := bg.Next()
	// peerA requests and waits for block alpha
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()
	alphaPromise, err := peerA.Exchange.GetBlocks(ctx, []*cid.Cid{alpha.Cid()})
	if err != nil {
		t.Fatal(err)
	}

	// peerB announces to the network that he has block alpha
	err = peerB.Exchange.HasBlock(alpha)
	if err != nil {
		t.Fatal(err)
	}

	// At some point, peerA should get alpha (or timeout)
	blkrecvd, ok := <-alphaPromise
	if !ok {
		t.Fatal("context timed out and broke promise channel!")
	}

	if !blkrecvd.Cid().Equals(alpha.Cid()) {
		t.Fatal("Wrong block!")
	}

}

func TestEmptyKey(t *testing.T) {
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
	sg := NewTestSessionGenerator(net)
	defer sg.Close()
	bs := sg.Instances(1)[0].Exchange

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	_, err := bs.GetBlock(ctx, nil)
	if err != blockstore.ErrNotFound {
		t.Error("empty str key should return ErrNotFound")
	}
}

func assertStat(st *Stat, sblks, rblks int, sdata, rdata uint64) error {
	if sblks != st.BlocksSent {
		return fmt.Errorf("mismatch in blocks sent: %d vs %d", sblks, st.BlocksSent)
	}

	if rblks != st.BlocksReceived {
		return fmt.Errorf("mismatch in blocks recvd: %d vs %d", rblks, st.BlocksReceived)
	}

	if sdata != st.DataSent {
		return fmt.Errorf("mismatch in data sent: %d vs %d", sdata, st.DataSent)
	}

	if rdata != st.DataReceived {
		return fmt.Errorf("mismatch in data recvd: %d vs %d", rdata, st.DataReceived)
	}
	return nil
}

func TestBasicBitswap(t *testing.T) {
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
	sg := NewTestSessionGenerator(net)
	defer sg.Close()
	bg := blocksutil.NewBlockGenerator()

	t.Log("Test a one node trying to get one block from another")

	instances := sg.Instances(2)
	blocks := bg.Blocks(1)
	err := instances[0].Exchange.HasBlock(blocks[0])
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	blk, err := instances[1].Exchange.GetBlock(ctx, blocks[0].Cid())
	if err != nil {
		t.Fatal(err)
	}

	st0, err := instances[0].Exchange.Stat()
	if err != nil {
		t.Fatal(err)
	}

	st1, err := instances[1].Exchange.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if err := assertStat(st0, 1, 0, 1, 0); err != nil {
		t.Fatal(err)
	}

	if err := assertStat(st1, 0, 1, 0, 1); err != nil {
		t.Fatal(err)
	}

	t.Log(blk)
	for _, inst := range instances {
		err := inst.Exchange.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDoubleGet(t *testing.T) {
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
	sg := NewTestSessionGenerator(net)
	defer sg.Close()
	bg := blocksutil.NewBlockGenerator()

	t.Log("Test a one node trying to get one block from another")

	instances := sg.Instances(2)
	blocks := bg.Blocks(1)

	ctx1, cancel1 := context.WithCancel(context.Background())
	blkch1, err := instances[1].Exchange.GetBlocks(ctx1, []*cid.Cid{blocks[0].Cid()})
	if err != nil {
		t.Fatal(err)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	blkch2, err := instances[1].Exchange.GetBlocks(ctx2, []*cid.Cid{blocks[0].Cid()})
	if err != nil {
		t.Fatal(err)
	}

	// ensure both requests make it into the wantlist at the same time
	time.Sleep(time.Millisecond * 100)
	cancel1()

	_, ok := <-blkch1
	if ok {
		t.Fatal("expected channel to be closed")
	}

	err = instances[0].Exchange.HasBlock(blocks[0])
	if err != nil {
		t.Fatal(err)
	}

	select {
	case blk, ok := <-blkch2:
		if !ok {
			t.Fatal("expected to get the block here")
		}
		t.Log(blk)
	case <-time.After(time.Second * 5):
		t.Fatal("timed out waiting on block")
	}

	for _, inst := range instances {
		err := inst.Exchange.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestWantlistCleanup(t *testing.T) {
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
	sg := NewTestSessionGenerator(net)
	defer sg.Close()
	bg := blocksutil.NewBlockGenerator()

	instances := sg.Instances(1)[0]
	bswap := instances.Exchange
	blocks := bg.Blocks(20)

	var keys []*cid.Cid
	for _, b := range blocks {
		keys = append(keys, b.Cid())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	_, err := bswap.GetBlock(ctx, keys[0])
	if err != context.DeadlineExceeded {
		t.Fatal("shouldnt have fetched any blocks")
	}

	time.Sleep(time.Millisecond * 50)

	if len(bswap.GetWantlist()) > 0 {
		t.Fatal("should not have anyting in wantlist")
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	_, err = bswap.GetBlocks(ctx, keys[:10])
	if err != nil {
		t.Fatal(err)
	}

	<-ctx.Done()
	time.Sleep(time.Millisecond * 50)

	if len(bswap.GetWantlist()) > 0 {
		t.Fatal("should not have anyting in wantlist")
	}

	_, err = bswap.GetBlocks(context.Background(), keys[:1])
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	_, err = bswap.GetBlocks(ctx, keys[10:])
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 50)
	if len(bswap.GetWantlist()) != 11 {
		t.Fatal("should have 11 keys in wantlist")
	}

	cancel()
	time.Sleep(time.Millisecond * 50)
	if !(len(bswap.GetWantlist()) == 1 && bswap.GetWantlist()[0] == keys[0]) {
		t.Fatal("should only have keys[0] in wantlist")
	}
}
