package bitswap

import (
	"code.google.com/p/goprotobuf/proto"
	blocks "github.com/jbenet/go-ipfs/blocks"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"

	ds "github.com/jbenet/datastore.go"

	"time"
)

// PartnerWantListMax is the bound for the number of keys we'll store per
// partner. These are usually taken from the top of the Partner's WantList
// advertisements. WantLists are sorted in terms of priority.
const PartnerWantListMax = 10

// KeySet is just a convenient alias for maps of keys, where we only care
// access/lookups.
type KeySet map[u.Key]struct{}

// BitSwap instances implement the bitswap protocol.
type BitSwap struct {
	// peer is the identity of this (local) node.
	peer *peer.Peer

	// net holds the connections to all peers.
	net     swarm.Network
	meschan *swarm.Chan

	// datastore is the local database // Ledgers of known
	datastore ds.Datastore

	// routing interface for communication
	routing *dht.IpfsDHT

	listener *swarm.MessageListener

	// partners is a map of currently active bitswap relationships.
	// The Ledger has the peer.ID, and the peer connection works through net.
	// Ledgers of known relationships (active or inactive) stored in datastore.
	// Changes to the Ledger should be committed to the datastore.
	partners map[u.Key]*Ledger

	// haveList is the set of keys we have values for. a map for fast lookups.
	// haveList KeySet -- not needed. all values in datastore?

	// wantList is the set of keys we want values for. a map for fast lookups.
	wantList KeySet

	haltChan chan struct{}
}

// NewBitSwap creates a new BitSwap instance. It does not check its parameters.
func NewBitSwap(p *peer.Peer, net swarm.Network, d ds.Datastore, r routing.IpfsRouting) *BitSwap {
	bs := &BitSwap{
		peer:      p,
		net:       net,
		datastore: d,
		partners:  LedgerMap{},
		wantList:  KeySet{},
		routing:   r.(*dht.IpfsDHT),
		meschan:   net.GetChannel(swarm.PBWrapper_BITSWAP),
		haltChan:  make(chan struct{}),
		listener:  swarm.NewMessageListener(),
	}

	go bs.handleMessages()
	return bs
}

// GetBlock attempts to retrieve a particular block from peers, within timeout.
func (bs *BitSwap) GetBlock(k u.Key, timeout time.Duration) (
	*blocks.Block, error) {
	begin := time.Now()
	tleft := timeout - time.Now().Sub(begin)
	provs_ch := bs.routing.FindProvidersAsync(k, 20, timeout)

	valchan := make(chan []byte)
	after := time.After(tleft)

	// TODO: when the data is received, shut down this for loop ASAP
	go func() {
		for p := range provs_ch {
			go func(pr *peer.Peer) {
				ledger := bs.GetLedger(pr)
				blk, err := bs.getBlock(k, pr, tleft)
				if err != nil {
					u.PErr("getBlock returned: %v\n", err)
					return
				}
				// NOTE: this credits everyone who sends us a block,
				//       even if we dont use it
				ledger.ReceivedBytes(uint64(len(blk)))
				select {
				case valchan <- blk:
				default:
				}
			}(p)
		}
	}()

	select {
	case blkdata := <-valchan:
		close(valchan)
		return blocks.NewBlock(blkdata)
	case <-after:
		return nil, u.ErrTimeout
	}
}

func (bs *BitSwap) getBlock(k u.Key, p *peer.Peer, timeout time.Duration) ([]byte, error) {
	u.DOut("[%s] getBlock '%s' from [%s]\n", bs.peer.ID.Pretty(), k.Pretty(), p.ID.Pretty())
	//
	mes := new(PBMessage)
	mes.Id = proto.Uint64(swarm.GenerateMessageID())
	mes.Key = proto.String(string(k))
	typ := PBMessage_GET_BLOCK
	mes.Type = &typ
	//

	after := time.After(timeout)
	resp := bs.listener.Listen(mes.GetId(), 1, timeout)
	smes := swarm.NewMessage(p, mes)
	bs.meschan.Outgoing <- smes

	select {
	case resp_mes := <-resp:
		pmes := new(PBMessage)
		err := proto.Unmarshal(resp_mes.Data, pmes)
		if err != nil {
			return nil, err
		}
		if pmes.GetSuccess() {
			return pmes.GetValue(), nil
		}
		return nil, u.ErrNotFound
	case <-after:
		u.PErr("getBlock for '%s' timed out.\n", k)
		return nil, u.ErrTimeout
	}
}

// HaveBlock announces the existance of a block to BitSwap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (bs *BitSwap) HaveBlock(k u.Key) error {
	return bs.routing.Provide(k)
}

func (bs *BitSwap) handleMessages() {
	for {
		select {
		case mes := <-bs.meschan.Incoming:
			pmes := new(PBMessage)
			err := proto.Unmarshal(mes.Data, pmes)
			if err != nil {
				u.PErr("%v\n", err)
				continue
			}
			if pmes.GetResponse() {
				bs.listener.Respond(pmes.GetId(), mes)
				continue
			}

			switch pmes.GetType() {
			case PBMessage_GET_BLOCK:
				go bs.handleGetBlock(mes.Peer, pmes)
			case PBMessage_WANT_BLOCK:
				go bs.handleWantBlock(mes.Peer, pmes)
			default:
				u.PErr("Invalid message type.\n")
			}
		case <-bs.haltChan:
			return
		}
	}
}

func (bs *BitSwap) handleWantBlock(p *peer.Peer, pmes *PBMessage) {
	wants := pmes.GetWantlist()
	ledg := bs.GetLedger(p)
	for _, s := range wants {
		// TODO: this needs to be different. We need timeouts.
		ledg.WantList[u.Key(s)] = struct{}{}
	}
}

func (bs *BitSwap) handleGetBlock(p *peer.Peer, pmes *PBMessage) {
	u.DOut("handleGetBlock.\n")
	ledger := bs.GetLedger(p)

	u.DOut("finding [%s] in datastore.\n", u.Key(pmes.GetKey()).Pretty())
	idata, err := bs.datastore.Get(ds.NewKey(pmes.GetKey()))
	if err != nil {
		u.PErr("handleGetBlock datastore returned: %v\n", err)
		if err == ds.ErrNotFound {
			return
		}
		return
	}

	u.DOut("found value!\n")
	data, ok := idata.([]byte)
	if !ok {
		u.PErr("Failed casting data from datastore.")
		return
	}

	if ledger.ShouldSend() {
		u.DOut("Sending value back!\n")
		resp := &Message{
			Value:    data,
			Response: true,
			ID:       pmes.GetId(),
			Type:     PBMessage_GET_BLOCK,
			Success:  true,
		}
		bs.meschan.Outgoing <- swarm.NewMessage(p, resp.ToProtobuf())
		ledger.SentBytes(uint64(len(data)))
	} else {
		u.DOut("Ledger decided not to send anything...\n")
	}
}

func (bs *BitSwap) GetLedger(p *peer.Peer) *Ledger {
	l, ok := bs.partners[p.Key()]
	if ok {
		return l
	}

	l = new(Ledger)
	l.Strategy = StandardStrategy
	l.Partner = p
	bs.partners[p.Key()] = l
	return l
}

func (bs *BitSwap) SendWantList(wl KeySet) error {
	mes := Message{
		ID:       swarm.GenerateMessageID(),
		Type:     PBMessage_WANT_BLOCK,
		WantList: bs.wantList,
	}

	pbmes := mes.ToProtobuf()
	// Lets just ping everybody all at once
	for _, ledger := range bs.partners {
		bs.meschan.Outgoing <- swarm.NewMessage(ledger.Partner, pbmes)
	}

	return nil
}

func (bs *BitSwap) Halt() {
	bs.haltChan <- struct{}{}
}
