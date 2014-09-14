package context

import (
	"errors"
	"sync"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func TestClientLogErrorThenReturnOnCancel(t *testing.T) {
	ctx, cancelFunc := WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go func(ctx Context) {
		ctx.LogError(errors.New("A mildly interesting event"))
		wg.Done()    // 0th
		<-ctx.Done() // 3rd
	}(ctx)

	wg.Wait()    // 1st
	cancelFunc() // 2nd
}
