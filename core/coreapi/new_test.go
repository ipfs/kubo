package coreapi

import (
	"context"
	"errors"
	"testing"

	"github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/tests"
)

var errSkip = errors.New("skip")

type makeSingle func() *CoreAPI

func (f makeSingle) MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]iface.CoreAPI, error) {
	if n != 1 {
		return nil, errSkip
	}

	return []iface.CoreAPI{
		f(),
	}, nil
}

func BasicTests(ts *tests.TestSuite, t *testing.T) {
	ts.TestKey(t)
	ts.TestUnixfs(t)
}

func TestNew(t *testing.T) {
	for _, testcase := range []struct {
		name string
		opts []Option
		test func(*tests.TestSuite, *testing.T)
	}{
		{
			name: "default",
			test: BasicTests,
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			f := func() *CoreAPI {
				api, err := New(testcase.opts...)
				if err != nil {
					t.Fatal(err)
				}

				if api == nil {
					t.Fatal("api is nil")
				}

				return api
			}

			ts := &tests.TestSuite{
				Provider: makeSingle(f),
			}

			testcase.test(ts, t)
		})
	}
}

var _ tests.Provider = makeSingle(nil)
