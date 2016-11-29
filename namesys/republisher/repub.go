package republisher

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/go-ipfs/namesys"

	"gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	gpctx "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	pstore "gx/ipfs/QmeXj9VAjmYQZxpmVz7VzccbJrpmr8qkCDSjfVNsPTWTYU/go-libp2p-peerstore"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
)

var log = logging.Logger("ipns-repub")

var DefaultRebroadcastInterval = time.Hour * 4

const DefaultRecordLifetime = time.Hour * 24

type Republisher struct {
	repub     namesys.RePublisher
	peerStore pstore.Peerstore

	Interval time.Duration

	// how long records that are republished should be valid for
	RecordLifetime time.Duration

	entrylock sync.Mutex
	entries   map[peer.ID]struct{}
}

func NewRepublisher(repub namesys.RePublisher, peerStore pstore.Peerstore) *Republisher {
	return &Republisher{
		repub:          repub,
		peerStore:      peerStore,
		entries:        make(map[peer.ID]struct{}),
		Interval:       DefaultRebroadcastInterval,
		RecordLifetime: DefaultRecordLifetime,
	}
}

func (rp *Republisher) AddName(id peer.ID) {
	rp.entrylock.Lock()
	defer rp.entrylock.Unlock()
	rp.entries[id] = struct{}{}
}

func (rp *Republisher) Run(proc goprocess.Process) {
	tick := time.NewTicker(rp.Interval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			err := rp.republishEntries(proc)
			if err != nil {
				log.Error("Republisher failed to republish: ", err)
			}
		case <-proc.Closing():
			return
		}
	}
}

func (rp *Republisher) republishEntries(proc goprocess.Process) error {
	ctx, cancel := context.WithCancel(gpctx.OnClosingContext(proc))
	defer cancel()

	for id := range rp.entries {
		log.Debugf("republishing ipns entry for %s", id)
		sk := rp.peerStore.PrivKey(id)
		eol := time.Now().Add(rp.RecordLifetime)

		err := rp.repub.RePublish(ctx, sk, eol)
		if err != nil {
			return err
		}
	}

	return nil
}
