package republisher

import (
	"errors"
	"sync"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	namesys "github.com/ipfs/go-ipfs/namesys"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	"github.com/ipfs/go-ipfs/routing"
	dhtpb "github.com/ipfs/go-ipfs/routing/dht/pb"

	peer "gx/ipfs/QmQGwpJy9P4yXZySmqkZEXCmbBpJUb8xntCv8Ca4taZwDC/go-libp2p-peer"
	goprocess "gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess"
	gpctx "gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess/context"
	pstore "gx/ipfs/QmXHUpFsnpCmanRnacqYkFoLoFfEq5yS2nUgGkAjJ1Nj9j/go-libp2p-peerstore"
	logging "gx/ipfs/QmYtB7Qge8cJpXc4irsEp8zRqfnZMBeB7aTrMEkPk67DRv/go-log"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var errNoEntry = errors.New("no previous entry")

var log = logging.Logger("ipns-repub")

var DefaultRebroadcastInterval = time.Hour * 4

const DefaultRecordLifetime = time.Hour * 24

type Republisher struct {
	r  routing.IpfsRouting
	ds ds.Datastore
	ps pstore.Peerstore

	Interval time.Duration

	// how long records that are republished should be valid for
	RecordLifetime time.Duration

	entrylock sync.Mutex
	entries   map[peer.ID]struct{}
}

func NewRepublisher(r routing.IpfsRouting, ds ds.Datastore, ps pstore.Peerstore) *Republisher {
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

func (rp *Republisher) getLastVal(k key.Key) (path.Path, uint64, error) {
	ival, err := rp.ds.Get(k.DsKey())
	if err != nil {
		// not found means we dont have a previously published entry
		return "", 0, errNoEntry
	}

	val := ival.([]byte)
	dhtrec := new(dhtpb.Record)
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
