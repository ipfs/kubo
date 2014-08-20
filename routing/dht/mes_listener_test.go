package dht

import (
	"testing"
	"time"

	"github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/swarm"
)

// Ensure that the Message Listeners basic functionality works
func TestMesListenerBasic(t *testing.T) {
	ml := newMesListener()
	a := GenerateMessageID()
	resp := ml.Listen(a, 1, time.Minute)

	pmes := new(swarm.PBWrapper)
	pmes.Message = []byte("Hello")
	pmes.Type = new(swarm.PBWrapper_MessageType)
	mes := swarm.NewMessage(new(peer.Peer), pmes)

	go ml.Respond(a, mes)

	del := time.After(time.Millisecond * 10)
	select {
	case get := <-resp:
		if string(get.Data) != string(mes.Data) {
			t.Fatal("Something got really messed up")
		}
	case <-del:
		t.Fatal("Waiting on message response timed out.")
	}
}
