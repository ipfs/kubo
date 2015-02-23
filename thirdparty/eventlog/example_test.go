package eventlog

import "golang.org/x/net/context"

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
