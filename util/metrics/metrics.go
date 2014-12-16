package metrics

import "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/rcrowley/go-metrics"

// TODO add a metrics registry exposing functionality of the form M.RegisterBWCounter("system", &bwc)

type BandwidthCounter interface {
	IncIn(i int64)
	IncOut(i int64)
	InOut() (int64, int64)
}

func NewBandwidthCounter() BandwidthCounter {
	return &bwc{}
}

type bwc struct {
	in  metrics.StandardCounter
	out metrics.StandardCounter
}

func (c *bwc) IncIn(i int64) {
	c.in.Inc(i)
}

func (c *bwc) IncOut(i int64) {
	c.out.Inc(i)
}

func (c *bwc) InOut() (int64, int64) {
	return c.in.Count(), c.out.Count()
}
