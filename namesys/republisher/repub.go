package republisher

import (
	"context"
	"errors"
	"sync"
	"time"

	namesys "github.com/ipfs/go-ipfs/namesys"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"

	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	goprocess "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	gpctx "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	routing "gx/ipfs/QmbkGVaN9W6RYJK4Ws5FvMKXKDqdRQ5snhtaa92qP6L8eU/go-libp2p-routing"
	recpb "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record/pb"
	pstore "gx/ipfs/QmeXj9VAjmYQZxpmVz7VzccbJrpmr8qkCDSjfVNsPTWTYU/go-libp2p-peerstore"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
)

var errNoEntry = errors.New("no previous entry")

var log = logging.Logger("ipns-repub")

var DefaultRebroadcastInterval = time.Hour * 4

const DefaultRecordLifetime = time.Hour * 24 * 7

type Republisher struct {
	r  routing.ValueStore
	ds ds.Datastore
	ps pstore.Peerstore

	Interval time.Duration

	// how long records that are republished should be valid for
	RecordLifetime time.Duration

	entrylock sync.Mutex
	entries   map[peer.ID]struct{}
}

func NewRepublisher(r routing.ValueStore, ds ds.Datastore, ps pstore.Peerstore) *Republisher {
	return &Republisher{
		r:              r,
		ps:             ps,
		ds:             ds,
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

func (rp *Republisher) republishEntries(p goprocess.Process) error {
	ctx, cancel := context.WithCancel(gpctx.OnClosingContext(p))
	defer cancel()

	for id, _ := range rp.entries {
		log.Debugf("republishing ipns entry for %s", id)
		priv := rp.ps.PrivKey(id)

		// Look for it locally only
		_, ipnskey := namesys.IpnsKeysForID(id)
		p, seq, err := rp.getLastVal(ipnskey)
		if err != nil {
			if err == errNoEntry {
				continue
			}
			return err
		}

		// update record with same sequence number
		eol := time.Now().Add(rp.RecordLifetime)
		err = namesys.PutRecordToRouting(ctx, priv, p, seq, eol, rp.r, id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rp *Republisher) getLastVal(k string) (path.Path, uint64, error) {
	ival, err := rp.ds.Get(dshelp.NewKeyFromBinary([]byte(k)))
	if err != nil {
		// not found means we dont have a previously published entry
		return "", 0, errNoEntry
	}

	val := ival.([]byte)
	dhtrec := new(recpb.Record)
	err = proto.Unmarshal(val, dhtrec)
	if err != nil {
		return "", 0, err
	}

	// extract published data from record
	e := new(pb.IpnsEntry)
	err = proto.Unmarshal(dhtrec.GetValue(), e)
	if err != nil {
		return "", 0, err
	}
	return path.Path(e.Value), e.GetSequence(), nil
}
