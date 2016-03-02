package testify

import (
	"gx/ipfs/QmZwjfAKWe7vWZ8f48u7AGA1xYfzR1iCD9A2XSCYFRBWot/testify/assert"
	"testing"
)

func TestImports(t *testing.T) {
	if assert.Equal(t, 1, 1) != true {
		t.Error("Something is wrong.")
	}
}
