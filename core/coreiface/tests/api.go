package tests

import (
	"context"
	"testing"
	"time"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
)

func (tp *provider) makeAPI(ctx context.Context) (coreiface.CoreAPI, error) {
	api, err := tp.MakeAPISwarm(ctx, false, 1)
	if err != nil {
		return nil, err
	}

	return api[0], nil
}

type Provider interface {
	// Make creates n nodes. fullIdentity set to false can be ignored
	MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]coreiface.CoreAPI, error)
}

func (tp *provider) MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]coreiface.CoreAPI, error) {
	tp.apis <- 1
	go func() {
		<-ctx.Done()
		tp.apis <- -1
	}()

	return tp.Provider.MakeAPISwarm(ctx, fullIdentity, n)
}

type provider struct {
	Provider

	apis chan int
}

func TestApi(p Provider) func(t *testing.T) {
	running := 1
	apis := make(chan int)
	zeroRunning := make(chan struct{})
	go func() {
		for i := range apis {
			running += i
			if running < 1 {
				close(zeroRunning)
				return
			}
		}
	}()

	tp := &provider{Provider: p, apis: apis}

	return func(t *testing.T) {
		t.Run("Block", tp.TestBlock)
		t.Run("Dag", tp.TestDag)
		t.Run("Dht", tp.TestDht)
		t.Run("Key", tp.TestKey)
		t.Run("Name", tp.TestName)
		t.Run("Object", tp.TestObject)
		t.Run("Path", tp.TestPath)
		t.Run("Pin", tp.TestPin)
		t.Run("PubSub", tp.TestPubSub)
		t.Run("Unixfs", tp.TestUnixfs)

		apis <- -1
		t.Run("TestsCancelCtx", func(t *testing.T) {
			select {
			case <-zeroRunning:
			case <-time.After(time.Second):
				t.Errorf("%d test swarms(s) not closed", running)
			}
		})
	}
}
