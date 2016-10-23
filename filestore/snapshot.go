package filestore

import (
	"sync"

	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb"
)

type Snapshot struct {
	*Basic
}

func (ss Snapshot) Defined() bool { return ss.Basic != nil }

func (b *Basic) Verify() VerifyWhen { return b.ds.verify }

func (d *Basic) DB() dbread { return d.db }

func (d *Datastore) DB() dbwrap { return d.db }

func (d *Datastore) AsBasic() *Basic { return &Basic{d.db.dbread, d} }

func (d *Basic) AsFull() *Datastore { return d.ds }

func (d *Datastore) GetSnapshot() (Snapshot, error) {
	if d.snapshot.Defined() {
		d.snapshotUsed = true
		return d.snapshot, nil
	}
	ss, err := d.db.db.GetSnapshot()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{&Basic{dbread{ss}, d}}, nil
}

func (d *Datastore) releaseSnapshot() {
	if !d.snapshot.Defined() {
		return
	}
	if !d.snapshotUsed {
		d.snapshot.db.db.(*leveldb.Snapshot).Release()
	}
	d.snapshot = Snapshot{}
}

func NoOpLocker() sync.Locker {
	return noopLocker{}
}

type noopLocker struct{}

func (l noopLocker) Lock() {}

func (l noopLocker) Unlock() {}

type addLocker struct {
	adders int
	lock   sync.Mutex
	ds     *Datastore
}

func (l *addLocker) Lock() {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.adders == 0 {
		l.ds.releaseSnapshot()
		l.ds.snapshot, _ = l.ds.GetSnapshot()
	}
	l.adders += 1
	log.Debugf("acquired add-lock refcnt now %d\n", l.adders)
}

func (l *addLocker) Unlock() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.adders -= 1
	if l.adders == 0 {
		l.ds.releaseSnapshot()
	}
	log.Debugf("released add-lock refcnt now %d\n", l.adders)
}

func (d *Datastore) AddLocker() sync.Locker {
	return &d.addLocker
}
