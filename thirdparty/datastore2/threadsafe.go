package datastore2

import (
	"io"

	"gx/ipfs/QmNgqJarToRiq2GBaPJhkmW4B5BxS5B74E1rkGvv2JoaTp/go-datastore"
)

// ClaimThreadSafe claims that a Datastore is threadsafe, even when
// it's type does not guarantee this. Use carefully.
type ClaimThreadSafe struct {
	datastore.Batching
}

var _ datastore.ThreadSafeDatastore = ClaimThreadSafe{}

func (ClaimThreadSafe) IsThreadSafe() {}

// TEMP UNTIL dev0.4.0 merges and solves this ugly interface stuff
func (c ClaimThreadSafe) Close() error {
	return c.Batching.(io.Closer).Close()
}
