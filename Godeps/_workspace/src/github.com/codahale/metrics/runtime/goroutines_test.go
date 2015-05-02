package runtime

import (
	"testing"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/codahale/metrics"
)

func TestGoroutinesStats(t *testing.T) {
	_, gauges := metrics.Snapshot()

	expected := []string{
		"Goroutines.Num",
	}

	for _, name := range expected {
		if _, ok := gauges[name]; !ok {
			t.Errorf("Missing gauge %q", name)
		}
	}
}
