package swarm

import (
	"math/rand"
	"sync"
	"time"

	u "github.com/jbenet/go-ipfs/util"
)

type MesListener struct {
	listeners map[uint64]*listenInfo
	haltchan  chan struct{}
	unlist    chan uint64
	nlist     chan *listenInfo
	send      chan *respMes
}

// GenerateMessageID creates and returns a new message ID
func GenerateMessageID() uint64 {
	return (uint64(rand.Uint32()) << 32) | uint64(rand.Uint32())
}

// The listen info struct holds information about a message that is being waited for
type listenInfo struct {
	// Responses matching the listen ID will be sent through resp
	resp chan *Message

	// count is the number of responses to listen for
	count int

	// eol is the time at which this listener will expire
	eol time.Time

	// sendlock is used to prevent conditions where we try to send on the resp
	// channel as its being closed by a timeout in another thread
	sendLock sync.Mutex

	closed bool

	id uint64
}

func NewMesListener() *MesListener {
	ml := new(MesListener)
	ml.haltchan = make(chan struct{})
	ml.listeners = make(map[uint64]*listenInfo)
	ml.nlist = make(chan *listenInfo, 16)
	ml.send = make(chan *respMes, 16)
	ml.unlist = make(chan uint64, 16)
	go ml.run()
	return ml
}

func (ml *MesListener) Listen(id uint64, count int, timeout time.Duration) <-chan *Message {
	li := new(listenInfo)
	li.count = count
	li.eol = time.Now().Add(timeout)
	li.resp = make(chan *Message, count)
	li.id = id
	ml.nlist <- li
	return li.resp
}

func (ml *MesListener) Unlisten(id uint64) {
	ml.unlist <- id
}

type respMes struct {
	id  uint64
	mes *Message
}

func (ml *MesListener) Respond(id uint64, mes *Message) {
	ml.send <- &respMes{
		id:  id,
		mes: mes,
	}
}

func (ml *MesListener) Halt() {
	ml.haltchan <- struct{}{}
}

func (ml *MesListener) run() {
	for {
		select {
		case <-ml.haltchan:
			return
		case id := <-ml.unlist:
			trg, ok := ml.listeners[id]
			if !ok {
				continue
			}
			close(trg.resp)
			delete(ml.listeners, id)
		case li := <-ml.nlist:
			ml.listeners[li.id] = li
		case s := <-ml.send:
			trg, ok := ml.listeners[s.id]
			if !ok {
				u.DOut("Send with no listener.")
				continue
			}

			if time.Now().After(trg.eol) {
				close(trg.resp)
				delete(ml.listeners, s.id)
				continue
			}

			trg.resp <- s.mes
			trg.count--

			if trg.count == 0 {
				close(trg.resp)
				delete(ml.listeners, s.id)
			}
		}
	}
}
