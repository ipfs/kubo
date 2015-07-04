package delay

import (
	"sync"
	"time"
)

// Delay makes it easy to add (threadsafe) configurable delays to other
// objects.
type D interface {
	Set(time.Duration) time.Duration
	Wait()
	Get() time.Duration
}

// Fixed returns a delay with fixed latency
func Fixed(t time.Duration) D {
	return &delay{t: t}
}

type delay struct {
	l sync.RWMutex
	t time.Duration
}

// TODO func Variable(time.Duration) D returns a delay with probablistic latency

func (d *delay) Set(t time.Duration) time.Duration {
	d.l.Lock()
	defer d.l.Unlock()
	prev := d.t
	d.t = t
	return prev
}

func (d *delay) Wait() {
	d.l.RLock()
	defer d.l.RUnlock()
	time.Sleep(d.t)
}

func (d *delay) Get() time.Duration {
	d.l.Lock()
	defer d.l.Unlock()
	return d.t
}
