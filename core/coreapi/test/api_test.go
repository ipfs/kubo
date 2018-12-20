package test

import (
	"github.com/ipfs/go-ipfs/core/coreapi/interface/tests"
	"testing"
)

func TestIface(t *testing.T) {
	tests.TestApi(t)
}
