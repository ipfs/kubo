package tests

import (
	"context"
	"testing"

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

type provider struct {
	Provider
}

func TestApi(p Provider) func(t *testing.T) {
	tp := &provider{p}

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
	}
}
