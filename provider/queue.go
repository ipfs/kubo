package provider

import (
	"context"
	"math"
	"strconv"
	"strings"

	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	namespace "github.com/ipfs/go-datastore/namespace"
	query "github.com/ipfs/go-datastore/query"
)

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
}

// NewQueue creates a queue for cids
func NewQueue(ctx context.Context, name string, ds datastore.Datastore) (*Queue, error) {
	namespaced := namespace.Wrap(ds, datastore.NewKey("/"+name+"/queue/"))
	head, tail, err := getQueueHeadTail(ctx, name, namespaced)
	if err != nil {
		return nil, err
	}
	q := &Queue{
		name:    name,
		ctx:     ctx,
		head:    head,
		tail:    tail,
		ds:      namespaced,
		dequeue: make(chan cid.Cid),
		enqueue: make(chan cid.Cid),
	}
	q.work()
	return q, nil
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

type entry struct {
	cid cid.Cid
	key datastore.Key
}

// Look for next Cid in the queue and return it. Skip over gaps and mangled data
func (q *Queue) nextEntry() (datastore.Key, cid.Cid) {
	for {
		if q.head >= q.tail {
			return datastore.Key{}, cid.Undef
		}

		key := q.queueKey(q.head)
		value, err := q.ds.Get(key)

		if err == datastore.ErrNotFound {
			log.Warningf("Error missing entry in queue: %s", key)
			q.head++ // move on
			continue
		} else if err != nil {
			log.Warningf("Error fetching from queue: %s", err)
			continue
		}

		c, err := cid.Parse(value)
		if err != nil {
			log.Warningf("Error marshalling Cid from queue: ", err)
			q.head++
			err = q.ds.Delete(key)
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
