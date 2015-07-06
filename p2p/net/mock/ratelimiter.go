package mocknet

import (
	"time"
)

type ratelimiter struct {
	bandwidth    float64       // bytes per nanosecond
	allowance    float64       // in bytes
	maxAllowance float64       // in bytes
	lastUpdate   time.Time     // when allowance was updated last
	count        int           // number of times rate limiting was applied
	duration     time.Duration // total delay introduced due to rate limiting
}

func NewRatelimiter(bandwidth float64) *ratelimiter {
	//  convert bandwidth to bytes per nanosecond
	b := bandwidth / float64(time.Second)
	return &ratelimiter{
		bandwidth:    b,
		allowance:    0,
		maxAllowance: bandwidth,
		lastUpdate:   time.Now(),
	}
}

func (r *ratelimiter) UpdateBandwidth(bandwidth float64) {
	b := bandwidth / float64(time.Second)
	r.bandwidth = b
	r.allowance = 0
	r.maxAllowance = bandwidth
	r.lastUpdate = time.Now()
}

func (r *ratelimiter) Limit(dataSize int) time.Duration {
	//  update time
	var duration time.Duration = time.Duration(0)
	if r.bandwidth == 0 {
		return duration
	}
	current := time.Now()
	elapsedTime := current.Sub(r.lastUpdate)
	r.lastUpdate = current

	allowance := r.allowance + float64(elapsedTime)*r.bandwidth
	//  allowance can't exceed bandwidth
	if allowance > r.maxAllowance {
		allowance = r.maxAllowance
	}

	allowance -= float64(dataSize)
	if allowance < 0 {
		//  sleep until allowance is back to 0
		duration = time.Duration(-allowance / r.bandwidth)
		allowance = 0
		//  rate limiting was applied, record stats
		r.count++
		r.duration += duration
	}

	r.allowance = allowance
	return duration
}
