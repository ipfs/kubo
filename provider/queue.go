package provider

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"

	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	namespace "github.com/ipfs/go-datastore/namespace"
	query "github.com/ipfs/go-datastore/query"
)

// Entry allows for the durability in the queue. When a cid is dequeued it is
// not removed from the datastore until you call Complete() on the entry you
// receive.
type Entry struct {
	cid   cid.Cid
	key   datastore.Key
	queue *Queue
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

	enqueueLock sync.Mutex
	ds          datastore.Datastore // Must be threadsafe

	dequeue  chan *Entry
	added chan struct{}
}

// NewQueue creates a queue for cids
func NewQueue(ctx context.Context, name string, ds datastore.Datastore) (*Queue, error) {
	namespaced := namespace.Wrap(ds, datastore.NewKey("/"+name+"/queue/"))
	head, tail, err := getQueueHeadTail(ctx, name, namespaced)
	if err != nil {
		return nil, err
	}
	q := &Queue{
		name:        name,
		ctx:         ctx,
		head:        head,
		tail:        tail,
		enqueueLock: sync.Mutex{},
		ds:          namespaced,
		dequeue:     make(chan *Entry),
		added:       make(chan struct{}),
	}
	return q, nil
}

// Enqueue puts a cid in the queue
func (q *Queue) Enqueue(cid cid.Cid) error {
	q.enqueueLock.Lock()
	defer q.enqueueLock.Unlock()

	nextKey := q.queueKey(q.tail)

	if err := q.ds.Put(nextKey, cid.Bytes()); err != nil {
		return err
	}

	q.tail++

	select {
		case q.added <- struct{}{}:
		case <-q.ctx.Done():
		default:
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
	go func() {
		for {
			if q.IsEmpty() {
				select {
				case <-q.ctx.Done():
					return
				case <-q.added:
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
				q.head++
				err = q.ds.Delete(entry.key)
			}
		}
	}()
}

// Find the next item in the queue, crawl forward if an entry is not
// found in the next spot.
func (q *Queue) next() (*Entry, error) {
	var key datastore.Key
	var value []byte
	var err error
	for {
		if q.head >= q.tail {
			return nil, errors.New("next: no more entries in queue returning")
		}
		select {
		case <-q.ctx.Done():
			return nil, nil
		default:
		}
		key = q.queueKey(q.head)

		value, err = q.ds.Get(key)

		value, err = q.ds.Get(key)
		if err == datastore.ErrNotFound {
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
		key:   key,
		queue: q,
	}

	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (q *Queue) queueKey(id uint64) datastore.Key {
	return datastore.NewKey(strconv.FormatUint(id, 10))
}

// crawl over the queue entries to find the head and tail
func getQueueHeadTail(ctx context.Context, name string, datastore datastore.Datastore) (uint64, uint64, error) {
	q := query.Query{}
	results, err := datastore.Query(q)
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
