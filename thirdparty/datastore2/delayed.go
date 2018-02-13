package datastore2

import (
	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	dsq "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore/query"
	delay "gx/ipfs/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
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

func (dds *delayed) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(dds), nil
}

var _ ds.Datastore = &delayed{}
