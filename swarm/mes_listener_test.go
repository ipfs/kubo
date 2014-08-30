package swarm

import (
	"testing"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
)

// Ensure that the Message Listeners basic functionality works
func TestMessageListener(t *testing.T) {
	ml := NewMessageListener()
	a := GenerateMessageID()
	resp := ml.Listen(a, 1, time.Minute)

	pmes := new(PBWrapper)
	pmes.Message = []byte("Hello")
	pmes.Type = new(PBWrapper_MessageType)
	mes := NewMessage(new(peer.Peer), pmes)

	go ml.Respond(a, mes)

	del := time.After(time.Millisecond * 100)
	select {
	case get := <-resp:
		if string(get.Data) != string(mes.Data) {
			t.Fatal("Something got really messed up")
		}
	case <-del:
		t.Fatal("Waiting on message response timed out.")
	}
}
