package bitswap

import (
	"bytes"
	"sync"
	"testing"
	"time"

	detectrace "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-detect-race"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	travis "github.com/ipfs/go-ipfs/util/testutil/ci/travis"

	blocks "github.com/ipfs/go-ipfs/blocks"
	blocksutil "github.com/ipfs/go-ipfs/blocks/blocksutil"
	key "github.com/ipfs/go-ipfs/blocks/key"
	tn "github.com/ipfs/go-ipfs/exchange/bitswap/testnet"
	p2ptestutil "github.com/ipfs/go-ipfs/p2p/test/util"
	mockrouting "github.com/ipfs/go-ipfs/routing/mock"
	delay "github.com/ipfs/go-ipfs/thirdparty/delay"
)

// FIXME the tests are really sensitive to the network delay. fix them to work
// well under varying conditions
const kNetworkDelay = 0 * time.Millisecond

func TestClose(t *testing.T) {
	vnet := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(kNetworkDelay))
	sesgen := NewTestSessionGenerator(vnet)
	defer sesgen.Close()
	bgen := blocksutil.NewBlockGenerator()

	block := bgen.Next()
	bitswap := sesgen.Next()

	bitswap.Exchange.Close()
	bitswap.Exchange.GetBlock(context.Background(), block.Key())
}

func TestProviderForKeyButNetworkCannotFind(t *testing.T) { // TODO revisit this

	rs := mockrouting.NewServer()
	net := tn.VirtualNetwork(rs, delay.Fixed(kNetworkDelay))
	g := NewTestSessionGenerator(net)
	defer g.Close()

	block := blocks.NewBlock([]byte("block"))
	pinfo := p2ptestutil.RandTestBogusIdentityOrFatal(t)
	rs.Client(pinfo).Provide(context.Background(), block.Key()) // but not on network

	solo := g.Next()
	defer solo.Exchange.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	_, err := solo.Exchange.GetBlock(ctx, block.Key())

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
	received, err := wantsBlock.Exchange.GetBlock(ctx, block.Key())
	if err != nil {
		t.Log(err)
		t.Fatal("Expected to succeed")
	}

	if !bytes.Equal(block.Data, received.Data) {
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

	var blkeys []key.Key
	first := instances[0]
	for _, b := range blocks {
		blkeys = append(blkeys, b.Key())
		first.Exchange.HasBlock(b)
	}

	t.Log("Distribute!")

	wg := sync.WaitGroup{}
	for _, inst := range instances[1:] {
		wg.Add(1)
		go func(inst Instance) {
			defer wg.Done()
			outch, err := inst.Exchange.GetBlocks(ctx, blkeys)
			if err != nil {
				t.Fatal(err)
			}
			for _ = range outch {
			}
		}(inst)
	}
	wg.Wait()

	t.Log("Verify!")

	for _, inst := range instances {
		for _, b := range blocks {
			if _, err := inst.Blockstore().Get(b.Key()); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func getOrFail(bitswap Instance, b *blocks.Block, t *testing.T, wg *sync.WaitGroup) {
	if _, err := bitswap.Blockstore().Get(b.Key()); err != nil {
		_, err := bitswap.Exchange.GetBlock(context.Background(), b.Key())
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
	alphaPromise, err := peerA.Exchange.GetBlocks(ctx, []key.Key{alpha.Key()})
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

	if blkrecvd.Key() != alpha.Key() {
		t.Fatal("Wrong block!")
	}

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
	blk, err := instances[1].Exchange.GetBlock(ctx, blocks[0].Key())
	if err != nil {
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
