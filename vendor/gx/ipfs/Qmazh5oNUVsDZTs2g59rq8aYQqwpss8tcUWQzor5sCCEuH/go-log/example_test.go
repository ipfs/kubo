package log

import "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

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
