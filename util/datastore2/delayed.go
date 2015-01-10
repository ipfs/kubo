package datastore2

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsq "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"

	delay "github.com/jbenet/go-ipfs/util/delay"
)

func WithDelay(ds ds.Datastore, delay delay.D) ds.Datastore {
	return &delayed{ds: ds, delay: delay}
}

type delayed struct {
	ds    ds.Datastore
	delay delay.D
}

func (dds *delayed) Put(key ds.Key, value interface{}) (err error) {
	dds.delay.Wait()
	return dds.ds.Put(key, value)
}

func (dds *delayed) Get(key ds.Key) (value interface{}, err error) {
	dds.delay.Wait()
	return dds.ds.Get(key)
}

func (dds *delayed) Has(key ds.Key) (exists bool, err error) {
	dds.delay.Wait()
	return dds.ds.Has(key)
}

func (dds *delayed) Delete(key ds.Key) (err error) {
	dds.delay.Wait()
	return dds.ds.Delete(key)
}

func (dds *delayed) Query(q dsq.Query) (dsq.Results, error) {
	dds.delay.Wait()
	return dds.ds.Query(q)
}

var _ ds.Datastore = &delayed{}
