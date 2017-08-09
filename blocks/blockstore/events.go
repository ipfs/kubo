package blockstore

import (
	"sync"

	cid "gx/ipfs/QmTprEaAA2A9bst5XH7exuyi5KzNMK3SEDNN8rBDnKWcUS/go-cid"
	blocks "gx/ipfs/QmVA4mafxbfH5aEvNz8fyoxC6J1xhAtw88B4GerPznSZBg/go-block-format"
)

// Event is a type that contains information used to fire the event
type Event struct {
	cid   *cid.Cid
	block blocks.Block
}

// EventID is describes type of event fired
type EventID int32

const (
	// EventPostPut is fired after block was added to the blockstore
	EventPostPut EventID = iota
)

// EventFunc is type of function that can be registered into event system
// if return value of the function is true, the event will be marked as handled
// and there is no need to process further handlers
type EventFunc func(*Event) (bool, error)

type eventSystem struct {
	lock sync.RWMutex
	// in future this can be replaced buy concurrent map
	handlers map[EventID][]EventFunc
}

func newEventSystem() *eventSystem {
	return &eventSystem{handlers: make(map[EventID][]EventFunc)}
}

func (es *eventSystem) addHandler(t EventID, ef EventFunc) {
	es.lock.Lock()
	defer es.lock.Unlock()
	hs, ok := es.handlers[t]
	if !ok {
		hs = []EventFunc{ef}
	} else {
		hs = append(hs, ef)
	}
	es.handlers[t] = hs
}

func (es *eventSystem) fireEvent(t EventID, e *Event) error {
	es.lock.RLock()
	hs, ok := es.handlers[t]
	es.lock.RUnlock()
	if !ok {
		return nil
	}
	for _, h := range hs {
		handled, err := h(e)
		if err != nil {
			return err
		}
		if handled {
			break
		}
	}
	return nil
}

func (es *eventSystem) fireMany(t EventID, evs []*Event) error {
	es.lock.RLock()
	hs, ok := es.handlers[t]
	es.lock.RUnlock()
	if !ok {
		return nil
	}

	for _, e := range evs {
		for _, h := range hs {
			handled, err := h(e)
			if err != nil {
				return err
			}
			if handled {
				break
			}
		}
	}
	return nil
}
