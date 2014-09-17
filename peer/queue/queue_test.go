package queue

import (
	"fmt"
	"testing"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func newPeer(id string) *peer.Peer {
	return &peer.Peer{ID: peer.ID(id)}
}

func TestQueue(t *testing.T) {

	p1 := newPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31")
	p2 := newPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a32")
	p3 := newPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33")
	p4 := newPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a34")
	p5 := newPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31")

	// these are the peer.IDs' XORKeySpace Key values:
	// [228 47 151 130 156 102 222 232 218 31 132 94 170 208 80 253 120 103 55 35 91 237 48 157 81 245 57 247 66 150 9 40]
	// [26 249 85 75 54 49 25 30 21 86 117 62 85 145 48 175 155 194 210 216 58 14 241 143 28 209 129 144 122 28 163 6]
	// [78 135 26 216 178 181 224 181 234 117 2 248 152 115 255 103 244 34 4 152 193 88 9 225 8 127 216 158 226 8 236 246]
	// [125 135 124 6 226 160 101 94 192 57 39 12 18 79 121 140 190 154 147 55 44 83 101 151 63 255 94 179 51 203 241 51]

	pq := NewXORDistancePQ(u.Key("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31"))
	pq.Enqueue(p3)
	pq.Enqueue(p1)
	pq.Enqueue(p2)
	pq.Enqueue(p4)
	pq.Enqueue(p5)
	pq.Enqueue(p1)

	// should come out as: p1, p4, p3, p2

	if d := pq.Dequeue(); d != p1 && d != p5 {
		t.Error("ordering failed")
	}

	if d := pq.Dequeue(); d != p1 && d != p5 {
		t.Error("ordering failed")
	}

	if d := pq.Dequeue(); d != p1 && d != p5 {
		t.Error("ordering failed")
	}

	if pq.Dequeue() != p4 {
		t.Error("ordering failed")
	}

	if pq.Dequeue() != p3 {
		t.Error("ordering failed")
	}

	if pq.Dequeue() != p2 {
		t.Error("ordering failed")
	}

}

func newPeerTime(t time.Time) *peer.Peer {
	s := fmt.Sprintf("hmmm time: %v", t)
	h, _ := u.Hash([]byte(s))
	return &peer.Peer{ID: peer.ID(h)}
}

func TestSyncQueue(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*2)

	pq := NewXORDistancePQ(u.Key("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31"))
	cq := NewChanQueue(ctx, pq)
	countIn := 0
	countOut := 0

	produce := func() {
		tick := time.Tick(time.Millisecond)
		for {
			select {
			case tim := <-tick:
				countIn++
				cq.EnqChan <- newPeerTime(tim)
			case <-ctx.Done():
				return
			}
		}
	}

	consume := func() {
		for {
			select {
			case <-cq.DeqChan:
				countOut++
			case <-ctx.Done():
				return
			}
		}
	}

	for i := 0; i < 10; i++ {
		go produce()
		go produce()
		go consume()
	}

	select {
	case <-ctx.Done():
	}

	if countIn != countOut {
		t.Errorf("didnt get them all out: %d/%d", countOut, countIn)
	}

}
