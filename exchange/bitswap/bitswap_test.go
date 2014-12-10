package bitswap

import (
	"bytes"
	"sync"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	blocks "github.com/jbenet/go-ipfs/blocks"
	blocksutil "github.com/jbenet/go-ipfs/blocks/blocksutil"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	mockrouting "github.com/jbenet/go-ipfs/routing/mock"
	u "github.com/jbenet/go-ipfs/util"
	delay "github.com/jbenet/go-ipfs/util/delay"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

// FIXME the tests are really sensitive to the network delay. fix them to work
// well under varying conditions
const kNetworkDelay = 0 * time.Millisecond

func TestClose(t *testing.T) {
	// TODO
	t.Skip("TODO Bitswap's Close implementation is a WIP")
	vnet := tn.VirtualNetwork(delay.Fixed(kNetworkDelay))
	rout := mockrouting.NewServer()
	sesgen := NewSessionGenerator(vnet, rout)
	defer sesgen.Stop()
	bgen := blocksutil.NewBlockGenerator()

	block := bgen.Next()
	bitswap := sesgen.Next()

	bitswap.Exchange.Close()
	bitswap.Exchange.GetBlock(context.Background(), block.Key())
}

func TestGetBlockTimeout(t *testing.T) {

	net := tn.VirtualNetwork(delay.Fixed(kNetworkDelay))
	rs := mockrouting.NewServer()
	g := NewSessionGenerator(net, rs)
	defer g.Stop()

	self := g.Next()

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	block := blocks.NewBlock([]byte("block"))
	_, err := self.Exchange.GetBlock(ctx, block.Key())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

func TestProviderForKeyButNetworkCannotFind(t *testing.T) {

	net := tn.VirtualNetwork(delay.Fixed(kNetworkDelay))
	rs := mockrouting.NewServer()
	g := NewSessionGenerator(net, rs)
	defer g.Stop()

	block := blocks.NewBlock([]byte("block"))
	rs.Client(testutil.NewPeerWithIDString("testing")).Provide(context.Background(), block.Key()) // but not on network

	solo := g.Next()
	defer solo.Exchange.Close()

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	_, err := solo.Exchange.GetBlock(ctx, block.Key())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

// TestGetBlockAfterRequesting...

func TestGetBlockFromPeerAfterPeerAnnounces(t *testing.T) {

	net := tn.VirtualNetwork(delay.Fixed(kNetworkDelay))
	rs := mockrouting.NewServer()
	block := blocks.NewBlock([]byte("block"))
	g := NewSessionGenerator(net, rs)
	defer g.Stop()

	hasBlock := g.Next()
	defer hasBlock.Exchange.Close()

	if err := hasBlock.Blockstore().Put(block); err != nil {
		t.Fatal(err)
	}
	if err := hasBlock.Exchange.HasBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}

	wantsBlock := g.Next()
	defer wantsBlock.Exchange.Close()

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
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
	t.Parallel()
	numInstances := 500
	numBlocks := 2
	PerformDistributionTest(t, numInstances, numBlocks)
}

func TestLargeFile(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()
	numInstances := 10
	numBlocks := 100
	PerformDistributionTest(t, numInstances, numBlocks)
}

func PerformDistributionTest(t *testing.T, numInstances, numBlocks int) {
	if testing.Short() {
		t.SkipNow()
	}
	net := tn.VirtualNetwork(delay.Fixed(kNetworkDelay))
	rs := mockrouting.NewServer()
	sg := NewSessionGenerator(net, rs)
	defer sg.Stop()
	bg := blocksutil.NewBlockGenerator()

	t.Log("Test a few nodes trying to get one file with a lot of blocks")

	instances := sg.Instances(numInstances)
	blocks := bg.Blocks(numBlocks)

	t.Log("Give the blocks to the first instance")

	var blkeys []u.Key
	first := instances[0]
	for _, b := range blocks {
		first.Blockstore().Put(b)
		blkeys = append(blkeys, b.Key())
		first.Exchange.HasBlock(context.Background(), b)
		rs.Client(first.Peer).Provide(context.Background(), b.Key())
	}

	t.Log("Distribute!")

	wg := sync.WaitGroup{}
	for _, inst := range instances {
		wg.Add(1)
		go func(inst Instance) {
			defer wg.Done()
			outch, err := inst.Exchange.GetBlocks(context.TODO(), blkeys)
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

	net := tn.VirtualNetwork(delay.Fixed(kNetworkDelay))
	rs := mockrouting.NewServer()
	sg := NewSessionGenerator(net, rs)
	defer sg.Stop()
	bg := blocksutil.NewBlockGenerator()

	oldVal := rebroadcastDelay
	rebroadcastDelay = time.Second / 2
	defer func() { rebroadcastDelay = oldVal }()

	peerA := sg.Next()
	peerB := sg.Next()

	t.Logf("Session %v\n", peerA.Peer)
	t.Logf("Session %v\n", peerB.Peer)

	timeout := time.Second
	waitTime := time.Second * 5

	alpha := bg.Next()
	// peerA requests and waits for block alpha
	ctx, _ := context.WithTimeout(context.TODO(), waitTime)
	alphaPromise, err := peerA.Exchange.GetBlocks(ctx, []u.Key{alpha.Key()})
	if err != nil {
		t.Fatal(err)
	}

	// peerB announces to the network that he has block alpha
	ctx, _ = context.WithTimeout(context.TODO(), timeout)
	err = peerB.Exchange.HasBlock(ctx, alpha)
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
	net := tn.VirtualNetwork(delay.Fixed(kNetworkDelay))
	rs := mockrouting.NewServer()
	sg := NewSessionGenerator(net, rs)
	bg := blocksutil.NewBlockGenerator()

	t.Log("Test a few nodes trying to get one file with a lot of blocks")

	instances := sg.Instances(2)
	blocks := bg.Blocks(1)
	err := instances[0].Exchange.HasBlock(context.TODO(), blocks[0])
	if err != nil {
		t.Fatal(err)
	}

	ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
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
