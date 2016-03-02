package iter_test

import (
	"fmt"
	"testing"

	"gx/ipfs/QmYWL7Pyx6QHHryhLq96wR6CWidApH2D2nbXeTJbAmusH9/iter"
)

func ExampleN() {
	for i := range iter.N(4) {
		fmt.Println(i)
	}
	// Output:
	// 0
	// 1
	// 2
	// 3
}

func TestAllocs(t *testing.T) {
	var x []struct{}
	allocs := testing.AllocsPerRun(500, func() {
		x = iter.N(1e9)
	})
	if allocs > 0.1 {
		t.Errorf("allocs = %v", allocs)
	}
}
