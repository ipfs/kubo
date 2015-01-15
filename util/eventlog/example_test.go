package eventlog

import "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

func ExampleEventLogger() {
	{
		log := EventLogger(nil)
		e := log.EventBegin(context.Background(), "dial")
		e.Done()
	}
	{
		log := EventLogger(nil)
		e := log.EventBegin(context.Background(), "dial")
		_ = e.Close() // implements io.Closer for convenience
	}
}
