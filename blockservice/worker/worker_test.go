package worker

import "testing"

func TestStartClose(t *testing.T) {
	numRuns := 50
	if testing.Short() {
		numRuns = 5
	}
	for i := 0; i < numRuns; i++ {
		w := NewWorker(nil, DefaultConfig)
		w.Close()
	}
}
