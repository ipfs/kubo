package waitable

import (
	context "golang.org/x/net/context"
)

type Waitable interface {
	Closing() <-chan struct{}
}

// Context returns a context that cancels when the waitable is closing.
func Context(w Waitable) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-w.Closing()
		cancel()
	}()
	return ctx
}
