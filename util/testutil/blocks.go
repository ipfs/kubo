package testutil

import (
	"testing"

	blocks "github.com/jbenet/go-ipfs/blocks"
)

// NewBlockOrFail returns a block created from msgData. Signals test failure if
// creation fails.
//
// NB: NewBlockOrFail accepts a msgData parameter to avoid non-determinism in
// tests. Generating random block data could potentially result in unexpected
// behavior in tests. Thus, it is left up to the caller to select the msgData
// that will determine the blocks key.
func NewBlockOrFail(t *testing.T, msgData string) blocks.Block {
	block, blockCreationErr := blocks.NewBlock([]byte(msgData))
	if blockCreationErr != nil {
		t.Fatal(blockCreationErr)
	}
	return *block
}
