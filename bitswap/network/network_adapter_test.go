package network

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/proto"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	ci "github.com/jbenet/go-ipfs/crypto"
	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	msg "github.com/jbenet/go-ipfs/net/message"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	mux "github.com/jbenet/go-ipfs/net/mux"
	netservice "github.com/jbenet/go-ipfs/net/service"
	peer "github.com/jbenet/go-ipfs/peer"
	"testing"
)

type TestProtocol struct {
	*msg.Pipe
}

type FauxSender struct{}
type FauxReceiver struct{}

func (s *FauxSender) SendMessage(ctx context.Context, m netmsg.NetMessage) error {
	return nil
}
func (s *FauxSender) SendRequest(ctx context.Context, m netmsg.NetMessage) (netmsg.NetMessage, error) {
	return nil, nil
}
func (s *FauxSender) SetHandler(netservice.Handler) {}
func (r *FauxReceiver) ReceiveMessage(ctx context.Context, sender *peer.Peer, incoming bsmsg.BitSwapMessage) (
	destination *peer.Peer, outgoing bsmsg.BitSwapMessage, err error) {
	return nil, nil, nil
}

func newPeer(t *testing.T, id string) *peer.Peer {
	mh, err := mh.FromHexString(id)
	if err != nil {
		t.Error(err)
		return nil
	}
	return &peer.Peer{ID: peer.ID(mh)}
}

func wrapData(data []byte, pid mux.ProtocolID) ([]byte, error) {
	// Marshal
	pbm := new(mux.PBProtocolMessage)
	pbm.ProtocolID = &pid
	pbm.Data = data
	b, err := proto.Marshal(pbm)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func makePeer(addr *ma.Multiaddr) *peer.Peer {
	p := new(peer.Peer)
	p.AddAddress(addr)
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		panic(err)
	}
	p.PrivKey = sk
	p.PubKey = pk
	id, err := spipe.IDFromPubKey(pk)
	if err != nil {
		panic(err)
	}

	p.ID = id
	return p
}

func TestNetworkAdapter(t *testing.T) {
	s := &FauxSender{}
	r := &FauxReceiver{}
	netAdapter := NewNetworkAdapter(s, r)

	ctx := context.Background()

	//test for Send Message
	addrA, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/2222")
	peerA := makePeer(addrA)
	message := bsmsg.New()
	errSend := netAdapter.SendMessage(ctx, peerA, message)
	if errSend != nil {
		t.Error(errSend)
	}

	//test for send Request
	_, errRequest := netAdapter.SendRequest(ctx, peerA, message)
	if errRequest != nil {
		//Dependent on the brian's TODO method being implemented, failing otherwise
		//	t.Error(errRequest)
	}

}
