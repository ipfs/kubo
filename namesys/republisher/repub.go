package republisher

import (
	"errors"
	"sync"
	"time"

	namesys "github.com/ipfs/go-ipfs/namesys"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	key "github.com/ipfs/go-key"
	recpb "github.com/libp2p/go-libp2p-record/pb"
	"github.com/libp2p/go-libp2p-routing"

	proto "github.com/gogo/protobuf/proto"
	ds "github.com/ipfs/go-datastore"
	peer "github.com/ipfs/go-libp2p-peer"
	pstore "github.com/ipfs/go-libp2p-peerstore"
	logging "github.com/ipfs/go-log"
	goprocess "github.com/jbenet/goprocess"
	gpctx "github.com/jbenet/goprocess/context"
	context "golang.org/x/net/context"
)

var errNoEntry = errors.New("no previous entry")

var log = logging.Logger("ipns-repub")

var DefaultRebroadcastInterval = time.Hour * 4

const DefaultRecordLifetime = time.Hour * 24

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

func (rp *Republisher) getLastVal(k key.Key) (path.Path, uint64, error) {
	ival, err := rp.ds.Get(k.DsKey())
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
