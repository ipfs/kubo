package queue

import (
	"context"
	"fmt"
	"time"

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
	ds      datastore.Datastore // Must be threadsafe
	dequeue chan cid.Cid
	enqueue chan cid.Cid
	close   context.CancelFunc
	closed  chan struct{}
}

// NewQueue creates a queue for cids
func NewQueue(ctx context.Context, name string, ds datastore.Datastore) (*Queue, error) {
	namespaced := namespace.Wrap(ds, datastore.NewKey("/"+name+"/queue/"))
	cancelCtx, cancel := context.WithCancel(ctx)
	q := &Queue{
		name:    name,
		ctx:     cancelCtx,
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
				head, e := q.getQueueHead()

				if e != nil {
					log.Errorf("error querying for head of queue: %s, stopping provider", e)
					return
				} else if head != nil {
					k = datastore.NewKey(head.Key)
					c, e = cid.Parse(head.Value)
					if e != nil {
						log.Warningf("error parsing queue entry cid with key (%s), removing it from queue: %s", head.Key, e)
						err := q.ds.Delete(k)
						if err != nil {
							log.Errorf("error deleting queue entry with key (%s), due to error (%s), stopping provider", head.Key, err)
							return
						}
						continue
					}
				} else {
					c = cid.Undef
				}
			}

			// If c != cid.Undef set dequeue and attempt write, otherwise wait for enqueue
			var dequeue chan cid.Cid
			if c != cid.Undef {
				dequeue = q.dequeue
			}

			select {
			case toQueue := <-q.enqueue:
				keyPath := fmt.Sprintf("%d/%s", time.Now().UnixNano(), c.String())
				nextKey := datastore.NewKey(keyPath)

				if err := q.ds.Put(nextKey, toQueue.Bytes()); err != nil {
					log.Errorf("Failed to enqueue cid: %s", err)
					continue
				}
			case dequeue <- c:
				err := q.ds.Delete(k)

				if err != nil {
					log.Errorf("Failed to delete queued cid %s with key %s: %s", c, k, err)
					continue
				}
				c = cid.Undef
			case <-q.ctx.Done():
				return
			}
		}
	}()
}

func (q *Queue) getQueueHead() (*query.Result, error) {
	qry := query.Query{Orders: []query.Order{query.OrderByKey{}}, Limit: 1}
	results, err := q.ds.Query(qry)
	if err != nil {
		return nil, err
	}
	defer results.Close()
	r, ok := results.NextSync()
	if !ok {
		return nil, nil
	}

	return &r, nil
}
