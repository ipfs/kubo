package memdb

import (
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/testutil"
)

func TestMemDB(t *testing.T) {
	testutil.RunSuite(t, "MemDB Suite")
}
