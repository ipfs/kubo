package provider

import (
	"context"
	"errors"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
	"math"
	"strconv"
	"strings"
	"sync"
)

// Entry allows for the durability in the queue. When a cid is dequeued it is
// not removed from the datastore until you call Complete() on the entry you
// receive.
type Entry struct {
	cid   cid.Cid
	key   ds.Key
	queue *Queue
}

// Complete the entry by removing it from the queue
func (e *Entry) Complete() error {
	return e.queue.remove(e.key)
}

// Queue provides a durable, FIFO interface to the datastore for storing cids
//
// Durability just means that cids in the process of being provided when a
// crash or shutdown occurs will still be in the queue when the node is
// brought back online.
type Queue struct {
	// used to differentiate queues in datastore
	// e.g. provider vs reprovider
	name string

	ctx context.Context

	tail uint64
	head uint64

	lock      sync.Mutex
	datastore ds.Datastore

	dequeue  chan *Entry
	notEmpty chan struct{}

	isRunning bool
}

// NewQueue creates a queue for cids
func NewQueue(ctx context.Context, name string, datastore ds.Datastore) (*Queue, error) {
	namespaced := namespace.Wrap(datastore, ds.NewKey("/"+name+"/queue/"))
	head, tail, err := getQueueHeadTail(ctx, name, namespaced)
	if err != nil {
		return nil, err
	}
	q := &Queue{
		name:      name,
		ctx:       ctx,
		head:      head,
		tail:      tail,
		lock:      sync.Mutex{},
		datastore: namespaced,
		dequeue:   make(chan *Entry),
		notEmpty:  make(chan struct{}),
		isRunning: false,
	}
	return q, nil
}

// Enqueue puts a cid in the queue
func (q *Queue) Enqueue(cid cid.Cid) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	wasEmpty := q.IsEmpty()

	nextKey := q.queueKey(q.tail)

	if err := q.datastore.Put(nextKey, cid.Bytes()); err != nil {
		return err
	}

	q.tail++

	if q.isRunning && wasEmpty {
		select {
		case q.notEmpty <- struct{}{}:
		case <-q.ctx.Done():
		}
	}

	return nil
}

// Dequeue returns a channel that if listened to will remove entries from the queue
func (q *Queue) Dequeue() <-chan *Entry {
	return q.dequeue
}

// IsEmpty returns whether or not the queue has any items
func (q *Queue) IsEmpty() bool {
	return (q.tail - q.head) == 0
}

// Run dequeues items when the dequeue channel is available to
// be written to.
func (q *Queue) Run() {
	q.isRunning = true
	go func() {
		for {
			select {
			case <-q.ctx.Done():
				return
			default:
			}
			if q.IsEmpty() {
				select {
				case <-q.ctx.Done():
					return
					// wait for a notEmpty message
				case <-q.notEmpty:
				}
			}

			entry, err := q.next()
			if err != nil {
				log.Warningf("Error Dequeue()-ing: %s, %s", entry, err)
				continue
			}

			select {
			case <-q.ctx.Done():
				return
			case q.dequeue <- entry:
			}
		}
	}()
}

// Find the next item in the queue, crawl forward if an entry is not
// found in the next spot.
func (q *Queue) next() (*Entry, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	var nextKey ds.Key
	var value []byte
	var err error
	for {
		if q.head >= q.tail {
			return nil, errors.New("no more entries in queue")
		}
		select {
		case <-q.ctx.Done():
			return nil, nil
		default:
		}
		nextKey = q.queueKey(q.head)
		value, err = q.datastore.Get(nextKey)
		if err == ds.ErrNotFound {
			q.head++
			continue
		} else if err != nil {
			return nil, err
		} else {
			break
		}
	}

	id, err := cid.Parse(value)
	if err != nil {
		return nil, err
	}

	entry := &Entry{
		cid:   id,
		key:   nextKey,
		queue: q,
	}

	q.head++

	return entry, nil
}

func (q *Queue) queueKey(id uint64) ds.Key {
	return ds.NewKey(strconv.FormatUint(id, 10))
}

// crawl over the queue entries to find the head and tail
func getQueueHeadTail(ctx context.Context, name string, datastore ds.Datastore) (uint64, uint64, error) {
	query := query.Query{}
	results, err := datastore.Query(query)
	if err != nil {
		return 0, 0, err
	}

	var tail uint64
	var head uint64 = math.MaxUint64
	for entry := range results.Next() {
		select {
		case <-ctx.Done():
			return 0, 0, nil
		default:
		}
		trimmed := strings.TrimPrefix(entry.Key, "/")
		id, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			return 0, 0, err
		}

		if id < head {
			head = id
		}

		if (id + 1) > tail {
			tail = (id + 1)
		}
	}
	if err := results.Close(); err != nil {
		return 0, 0, err
	}
	if head == math.MaxUint64 {
		head = 0
	}

	return head, tail, nil
}

func (q *Queue) remove(key ds.Key) error {
	return q.datastore.Delete(key)
}
