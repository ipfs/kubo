package datastore2

import (
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

func WithDelay(ds datastore.Datastore, delay delay.D) datastore.Datastore {
	return &delayed{ds: ds, delay: delay}
}

type delayed struct {
	ds    datastore.Datastore
	delay delay.D
}

func (dds *delayed) Put(key datastore.Key, value interface{}) (err error) {
	dds.delay.Wait()
	return dds.ds.Put(key, value)
}

func (dds *delayed) Get(key datastore.Key) (value interface{}, err error) {
	dds.delay.Wait()
	return dds.ds.Get(key)
}

func (dds *delayed) Has(key datastore.Key) (exists bool, err error) {
	dds.delay.Wait()
	return dds.ds.Has(key)
}

func (dds *delayed) Delete(key datastore.Key) (err error) {
	dds.delay.Wait()
	return dds.ds.Delete(key)
}

func (dds *delayed) KeyList() ([]datastore.Key, error) {
	dds.delay.Wait()
	return dds.ds.KeyList()
}

var _ datastore.Datastore = &delayed{}
