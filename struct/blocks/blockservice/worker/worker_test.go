package worker

import (
	"testing"

	blocks "github.com/jbenet/go-ipfs/struct/blocks"
)

func TestStartClose(t *testing.T) {
	numRuns := 50
	if testing.Short() {
		numRuns = 5
	}
	for i := 0; i < numRuns; i++ {
		w := NewWorker(nil, DefaultConfig)
		w.Close()
	}
}

func TestQueueDeduplication(t *testing.T) {
	numUniqBlocks := 5 // arbitrary

	var firstBatch []*blocks.Block
	for i := 0; i < numUniqBlocks; i++ {
		firstBatch = append(firstBatch, blockFromInt(i))
	}

	// to get different pointer values and prevent the implementation from
	// cheating. The impl must check equality using Key.
	var secondBatch []*blocks.Block
	for i := 0; i < numUniqBlocks; i++ {
		secondBatch = append(secondBatch, blockFromInt(i))
	}
	var workQueue BlockList

	for _, b := range append(firstBatch, secondBatch...) {
		workQueue.Push(b)
	}
	for i := 0; i < numUniqBlocks; i++ {
		b := workQueue.Pop()
		if b.Key() != firstBatch[i].Key() {
			t.Fatal("list is not FIFO")
		}
	}
	if b := workQueue.Pop(); b != nil {
		t.Fatal("the workQueue did not de-duplicate the blocks")
	}
}

func TestPushPopPushPop(t *testing.T) {
	var workQueue BlockList
	orig := blockFromInt(1)
	dup := blockFromInt(1)
	workQueue.PushFront(orig)
	workQueue.Pop()
	workQueue.Push(dup)
	if workQueue.Len() != 1 {
		t.Fatal("the block list's internal state is corrupt")
	}
}

func blockFromInt(i int) *blocks.Block {
	return blocks.NewBlock([]byte(string(i)))
}
