package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var apiNotImplemented = errors.New("api not implemented")

func (tp *TestSuite) makeAPI(ctx context.Context) (coreiface.CoreAPI, error) {
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

func (tp *TestSuite) MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]coreiface.CoreAPI, error) {
	if tp.apis != nil {
		tp.apis <- 1
		go func() {
			<-ctx.Done()
			tp.apis <- -1
		}()
	}

	return tp.Provider.MakeAPISwarm(ctx, fullIdentity, n)
}

type TestSuite struct {
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

	tp := &TestSuite{Provider: p, apis: apis}

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

func (tp *TestSuite) hasApi(t *testing.T, tf func(coreiface.CoreAPI) error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := tf(api); err != nil {
		t.Fatal(api)
	}
}
