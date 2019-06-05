package queue

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	namespace "github.com/ipfs/go-datastore/namespace"
	query "github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("provider.queue")

// Queue provides a durable, FIFO interface to the datastore for storing cids
//
// Durability just means that cids in the process of being provided when a
// crash or shutdown occurs will still be in the queue when the node is
// brought back online.
type Queue struct {
	// used to differentiate queues in datastore
	// e.g. provider vs reprovider
	name    string
	ctx     context.Context
	tail    uint64
	head    uint64
	ds      datastore.Datastore // Must be threadsafe
	dequeue chan cid.Cid
	enqueue chan cid.Cid
	close   context.CancelFunc
	closed  chan struct{}
}

// NewQueue creates a queue for cids
func NewQueue(ctx context.Context, name string, ds datastore.Datastore) (*Queue, error) {
	namespaced := namespace.Wrap(ds, datastore.NewKey("/"+name+"/queue/"))
	head, tail, err := getQueueHeadTail(ctx, namespaced)
	if err != nil {
		return nil, err
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	q := &Queue{
		name:    name,
		ctx:     cancelCtx,
		head:    head,
		tail:    tail,
		ds:      namespaced,
		dequeue: make(chan cid.Cid),
		enqueue: make(chan cid.Cid),
		close:   cancel,
		closed:  make(chan struct{}, 1),
	}
	q.work()
	return q, nil
}

// Close stops the queue
func (q *Queue) Close() error {
	q.close()
	<-q.closed
	return nil
}

// Enqueue puts a cid in the queue
func (q *Queue) Enqueue(cid cid.Cid) {
	select {
	case q.enqueue <- cid:
	case <-q.ctx.Done():
	}
}

// Dequeue returns a channel that if listened to will remove entries from the queue
func (q *Queue) Dequeue() <-chan cid.Cid {
	return q.dequeue
}

// Look for next Cid in the queue and return it. Skip over gaps and mangled data
func (q *Queue) nextEntry() (datastore.Key, cid.Cid) {
	for {
		if q.head >= q.tail {
			return datastore.Key{}, cid.Undef
		}

		key := q.queueKey(q.head)
		value, err := q.ds.Get(key)

		if err != nil {
			if err == datastore.ErrNotFound {
				log.Warningf("Error missing entry in queue: %s", key)
			} else {
				log.Errorf("Error fetching from queue: %s", err)
			}
			q.head++ // move on
			continue
		}

		c, err := cid.Parse(value)
		if err != nil {
			log.Warningf("Error marshalling Cid from queue: ", err)
			q.head++
			err = q.ds.Delete(key)
			if err != nil {
				log.Warningf("Provider queue failed to delete: %s", key)
			}
			continue
		}

		return key, c
	}
}

// Run dequeues and enqueues when available.
func (q *Queue) work() {
	go func() {
		var k datastore.Key = datastore.Key{}
		var c cid.Cid = cid.Undef

		defer func() {
			close(q.closed)
		}()

		for {
			if c == cid.Undef {
				k, c = q.nextEntry()
			}

			// If c != cid.Undef set dequeue and attempt write, otherwise wait for enqueue
			var dequeue chan cid.Cid
			if c != cid.Undef {
				dequeue = q.dequeue
			}

			select {
			case toQueue := <-q.enqueue:
				nextKey := q.queueKey(q.tail)

				if err := q.ds.Put(nextKey, toQueue.Bytes()); err != nil {
					log.Errorf("Failed to enqueue cid: %s", err)
					continue
				}

				q.tail++
			case dequeue <- c:
				err := q.ds.Delete(k)

				if err != nil {
					log.Errorf("Failed to delete queued cid %s with key %s: %s", c, k, err)
					continue
				}
				c = cid.Undef
				q.head++
			case <-q.ctx.Done():
				return
			}
		}
	}()
}

func (q *Queue) queueKey(id uint64) datastore.Key {
	s := fmt.Sprintf("%016X", id)
	return datastore.NewKey(s)
}

func getQueueHeadTail(ctx context.Context, datastore datastore.Datastore) (uint64, uint64, error) {
	head, err := getQueueHead(datastore)
	if err != nil {
		return 0, 0, err
	}
	tail, err := getQueueTail(datastore)
	if err != nil {
		return 0, 0, err
	}
	return head, tail, nil
}

func getQueueHead(ds datastore.Datastore) (uint64, error) {
	return getFirstIDByOrder(ds, query.OrderByKey{})
}

func getQueueTail(ds datastore.Datastore) (uint64, error) {
	tail, err := getFirstIDByOrder(ds, query.OrderByKeyDescending{})
	if err != nil {
		return 0, err
	}
	if tail > 0 {
		tail++
	}
	return tail, nil
}

func getFirstIDByOrder(ds datastore.Datastore, order query.Order) (uint64, error) {
	q := query.Query{Orders: []query.Order{order}}
	results, err := ds.Query(q)
	if err != nil {
		return 0, err
	}
	defer results.Close()
	r, ok := results.NextSync()
	if !ok {
		return 0, nil
	}
	trimmed := strings.TrimPrefix(r.Key, "/")
	id, err := strconv.ParseUint(trimmed, 16, 64)
	if err != nil {
		return 0, err
	}
	return id, nil
}
