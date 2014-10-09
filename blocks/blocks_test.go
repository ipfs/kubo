package blocks

import "testing"

func TestBlocksBasic(t *testing.T) {

	// Test empty data
	empty := []byte{}
	NewBlock(empty)

	// Test nil case
	NewBlock(nil)

	// Test some data
	NewBlock([]byte("Hello world!"))
}
