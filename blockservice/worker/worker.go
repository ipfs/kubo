// TODO FIXME name me
package worker

import (
	"container/list"
	"errors"
	"time"

	process "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	ratelimit "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/ratelimit"
	blocks "github.com/jbenet/go-ipfs/blocks"
	exchange "github.com/jbenet/go-ipfs/exchange"
	waitable "github.com/jbenet/go-ipfs/thirdparty/waitable"
	util "github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("blockservice")

var DefaultConfig = Config{
	NumWorkers:       1,
	ClientBufferSize: 0,
	WorkerBufferSize: 0,
}

type Config struct {
	// NumWorkers sets the number of background workers that provide blocks to
	// the exchange.
	NumWorkers int

	// ClientBufferSize allows clients of HasBlock to send up to
	// |ClientBufferSize| blocks without blocking.
	ClientBufferSize int

	// WorkerBufferSize can be used in conjunction with NumWorkers to reduce
	// communication-coordination within the worker.
	WorkerBufferSize int
}

// TODO FIXME name me
type Worker struct {
	// added accepts blocks from client
	added    chan *blocks.Block
	exchange exchange.Interface

	// workQueue is owned by the client worker
	// process manages life-cycle
	process process.Process
}

func NewWorker(e exchange.Interface, c Config) *Worker {
	if c.NumWorkers < 1 {
		c.NumWorkers = 1 // provide a sane default
	}
	w := &Worker{
		exchange: e,
		added:    make(chan *blocks.Block, c.ClientBufferSize),
		process:  process.WithParent(process.Background()), // internal management
	}
	w.start(c)
	return w
}

func (w *Worker) HasBlock(b *blocks.Block) error {
	select {
	case <-w.process.Closed():
		return errors.New("blockservice worker is closed")
	case w.added <- b:
		return nil
	}
}

func (w *Worker) Close() error {
	log.Debug("blockservice provide worker is shutting down...")
	return w.process.Close()
}

func (w *Worker) start(c Config) {

	workerChan := make(chan *blocks.Block, c.WorkerBufferSize)

	// clientWorker handles incoming blocks from |w.added| and sends to
	// |workerChan|. This will never block the client.
	w.process.Go(func(proc process.Process) {
		defer close(workerChan)

		var workQueue BlockList
		debugInfo := time.NewTicker(5 * time.Second)
		defer debugInfo.Stop()
		for {

			// take advantage of the fact that sending on nil channel always
			// blocks so that a message is only sent if a block exists
			sendToWorker := workerChan
			nextBlock := workQueue.Pop()
			if nextBlock == nil {
				sendToWorker = nil
			}

			select {

			// if worker is ready and there's a block to process, send the
			// block
			case sendToWorker <- nextBlock:
			case <-debugInfo.C:
				if workQueue.Len() > 0 {
					log.Debugf("%d blocks in blockservice provide queue...", workQueue.Len())
				}
			case block := <-w.added:
				if nextBlock != nil {
					workQueue.Push(nextBlock) // missed the chance to send it
				}
				// if the client sends another block, add it to the queue.
				workQueue.Push(block)
			case <-proc.Closing():
				return
			}
		}
	})

	// reads from |workerChan| until process closes
	w.process.Go(func(proc process.Process) {
		ctx := waitable.Context(proc) // shut down in-progress HasBlock when time to die
		limiter := ratelimit.NewRateLimiter(process.Background(), c.NumWorkers)
		defer limiter.Close()
		for {
			select {
			case <-proc.Closing():
				return
			case block, ok := <-workerChan:
				if !ok {
					return
				}
				limiter.LimitedGo(func(proc process.Process) {
					if err := w.exchange.HasBlock(ctx, block); err != nil {
						log.Infof("blockservice worker error: %s", err)
					}
				})
			}
		}
	})
}

type BlockList struct {
	list    list.List
	uniques map[util.Key]*list.Element
}

func (s *BlockList) PushFront(b *blocks.Block) {
	if s.uniques == nil {
		s.uniques = make(map[util.Key]*list.Element)
	}
	_, ok := s.uniques[b.Key()]
	if !ok {
		e := s.list.PushFront(b)
		s.uniques[b.Key()] = e
	}
}

func (s *BlockList) Push(b *blocks.Block) {
	if s.uniques == nil {
		s.uniques = make(map[util.Key]*list.Element)
	}
	_, ok := s.uniques[b.Key()]
	if !ok {
		e := s.list.PushBack(b)
		s.uniques[b.Key()] = e
	}
}

func (s *BlockList) Pop() *blocks.Block {
	if s.list.Len() == 0 {
		return nil
	}
	e := s.list.Front()
	s.list.Remove(e)
	b := e.Value.(*blocks.Block)
	delete(s.uniques, b.Key())
	return b
}

func (s *BlockList) Len() int {
	return s.list.Len()
}
