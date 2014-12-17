package metrics

import "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/rcrowley/go-metrics"

// TODO add a metrics registry exposing functionality of the form M.RegisterBWCounter("system", &bwc)

type Counter interface {
	Count() int64
	Inc(int64)
}

type BandwidthCounter struct {
	in  metrics.StandardCounter
	out metrics.StandardCounter
}

func (c *BandwidthCounter) BytesIn() Counter {
	return &c.in
}

func (c *BandwidthCounter) BytesOut() Counter {
	return &c.out
}
