package bitswap

import (
	"context"
	"time"

	notifications "github.com/ipfs/go-ipfs/exchange/bitswap/notifications"
	blocks "gx/ipfs/QmXxGS5QsUxpR3iqL5DjmsYPHR1Yz74siRQ4ChJqWFosMh/go-block-format"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	loggables "gx/ipfs/QmVesPmqbPp7xRGyY96tnBwzDtVV1nqv4SCVxo5zCqKyH8/go-libp2p-loggables"
	cid "gx/ipfs/Qma4RJSuh7mMeJQYCqMbKzekn6EwBo7HEs5AQYjVRMQATB/go-cid"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

const activeWantsLimit = 16

// Session holds state for an individual bitswap transfer operation.
// This allows bitswap to make smarter decisions about who to send wantlist
// info to, and who to request blocks from
type Session struct {
	ctx            context.Context
	tofetch        []*cid.Cid
	activePeers    map[peer.ID]struct{}
	activePeersArr []peer.ID

	bs         *Bitswap
	incoming   chan blkRecv
	newReqs    chan []*cid.Cid
	cancelKeys chan []*cid.Cid

	interest  *lru.Cache
	liveWants map[string]time.Time
	liveCnt   int

	tick          *time.Timer
	baseTickDelay time.Duration

	latTotal time.Duration
	fetchcnt int

	notif notifications.PubSub

	uuid logging.Loggable

	id uint64
}

// NewSession creates a new bitswap session whose lifetime is bounded by the
// given context
func (bs *Bitswap) NewSession(ctx context.Context) *Session {
	s := &Session{
		activePeers:   make(map[peer.ID]struct{}),
		liveWants:     make(map[string]time.Time),
		newReqs:       make(chan []*cid.Cid),
		cancelKeys:    make(chan []*cid.Cid),
		ctx:           ctx,
		bs:            bs,
		incoming:      make(chan blkRecv),
		notif:         notifications.New(),
		uuid:          loggables.Uuid("GetBlockRequest"),
		baseTickDelay: time.Millisecond * 500,
		id:            bs.getNextSessionID(),
	}

	cache, _ := lru.New(2048)
	s.interest = cache

	bs.sessLk.Lock()
	bs.sessions = append(bs.sessions, s)
	bs.sessLk.Unlock()

	go s.run(ctx)

	return s
}

type blkRecv struct {
	from peer.ID
	blk  blocks.Block
}

func (s *Session) receiveBlockFrom(from peer.ID, blk blocks.Block) {
	s.incoming <- blkRecv{from: from, blk: blk}
}

func (s *Session) interestedIn(c *cid.Cid) bool {
	return s.interest.Contains(c.KeyString())
}

const provSearchDelay = time.Second * 10

func (s *Session) addActivePeer(p peer.ID) {
	if _, ok := s.activePeers[p]; !ok {
		s.activePeers[p] = struct{}{}
		s.activePeersArr = append(s.activePeersArr, p)
	}
}

func (s *Session) resetTick() {
	if s.latTotal == 0 {
		s.tick.Reset(provSearchDelay)
	} else {
		avLat := s.latTotal / time.Duration(s.fetchcnt)
		s.tick.Reset(s.baseTickDelay + (3 * avLat))
	}
}

func (s *Session) run(ctx context.Context) {
	s.tick = time.NewTimer(provSearchDelay)
	newpeers := make(chan peer.ID, 16)
	for {
		select {
		case blk := <-s.incoming:
			s.tick.Stop()

			s.addActivePeer(blk.from)

			s.receiveBlock(ctx, blk.blk)

			s.resetTick()
		case keys := <-s.newReqs:
			for _, k := range keys {
				s.interest.Add(k.KeyString(), nil)
			}
			if s.liveCnt < activeWantsLimit {
				toadd := activeWantsLimit - s.liveCnt
				if toadd > len(keys) {
					toadd = len(keys)
				}
				s.liveCnt += toadd

				now := keys[:toadd]
				keys = keys[toadd:]

				s.wantBlocks(ctx, now)
			}
			s.tofetch = append(s.tofetch, keys...)
		case keys := <-s.cancelKeys:
			s.cancel(keys)

		case <-s.tick.C:
			var live []*cid.Cid
			for c := range s.liveWants {
				cs, _ := cid.Cast([]byte(c))
				live = append(live, cs)
				s.liveWants[c] = time.Now()
			}

			// Broadcast these keys to everyone we're connected to
			s.bs.wm.WantBlocks(ctx, live, nil, s.id)

			if len(live) > 0 {
				go func() {
					for p := range s.bs.network.FindProvidersAsync(ctx, live[0], 10) {
						newpeers <- p
					}
				}()
			}
			s.resetTick()
		case p := <-newpeers:
			s.addActivePeer(p)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Session) receiveBlock(ctx context.Context, blk blocks.Block) {
	ks := blk.Cid().KeyString()
	if _, ok := s.liveWants[ks]; ok {
		s.liveCnt--
		tval := s.liveWants[ks]
		s.latTotal += time.Since(tval)
		s.fetchcnt++
		delete(s.liveWants, ks)
		s.notif.Publish(blk)

		if len(s.tofetch) > 0 {
			next := s.tofetch[0:1]
			s.tofetch = s.tofetch[1:]
			s.wantBlocks(ctx, next)
		}
	}
}

func (s *Session) wantBlocks(ctx context.Context, ks []*cid.Cid) {
	for _, c := range ks {
		s.liveWants[c.KeyString()] = time.Now()
	}
	s.bs.wm.WantBlocks(ctx, ks, s.activePeersArr, s.id)
}

func (s *Session) cancel(keys []*cid.Cid) {
	sset := cid.NewSet()
	for _, c := range keys {
		sset.Add(c)
	}
	var i, j int
	for ; j < len(s.tofetch); j++ {
		if sset.Has(s.tofetch[j]) {
			continue
		}
		s.tofetch[i] = s.tofetch[j]
		i++
	}
	s.tofetch = s.tofetch[:i]
}

func (s *Session) cancelWants(keys []*cid.Cid) {
	s.cancelKeys <- keys
}

func (s *Session) fetch(ctx context.Context, keys []*cid.Cid) {
	select {
	case s.newReqs <- keys:
	case <-ctx.Done():
	}
}

// GetBlocks fetches a set of blocks within the context of this session and
// returns a channel that found blocks will be returned on. No order is
// guaranteed on the returned blocks.
func (s *Session) GetBlocks(ctx context.Context, keys []*cid.Cid) (<-chan blocks.Block, error) {
	ctx = logging.ContextWithLoggable(ctx, s.uuid)
	return getBlocksImpl(ctx, keys, s.notif, s.fetch, s.cancelWants)
}

// GetBlock fetches a single block
func (s *Session) GetBlock(parent context.Context, k *cid.Cid) (blocks.Block, error) {
	return getBlock(parent, k, s.GetBlocks)
}
