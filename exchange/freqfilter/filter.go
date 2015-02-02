package filter

import (
	"sync"

	u "github.com/jbenet/go-ipfs/util"
)

type FrequencyFilter interface {
	AddKey(e u.Key)
}

type basicFilter struct {
	log     map[u.Key]int
	events  []u.Key
	event_i int

	threshold int
	trigger   func(u.Key)

	lock sync.Mutex
}

func NewBasicFilter(qsize, thresh int, trigger func(u.Key)) FrequencyFilter {
	return &basicFilter{
		log:    make(map[u.Key]int),
		events: make([]u.Key, qsize),
	}
}

func (bf *basicFilter) AddKey(e u.Key) {
	bf.lock.Lock()
	out := bf.events[bf.event_i]
	bf.log[out]--
	if bf.log[out] == 0 {
		delete(bf.log, out)
	}
	bf.events[bf.event_i] = e
	bf.log[e]++

	dotrigger := bf.log[e] > bf.threshold
	bf.lock.Unlock()

	if dotrigger {
		bf.trigger(e)
	}
}
