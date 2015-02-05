package util

import (
	"errors"
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func TestDoReturnsContextErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan struct{})
	err := WithContext(ctx, func() error {
		cancel()
		ch <- struct{}{} // won't return
		return nil
	})
	if err != ctx.Err() {
		t.Fail()
	}
}

func TestDoReturnsFuncError(t *testing.T) {
	ctx := context.Background()
	expected := errors.New("expected to be returned by ContextDo")
	err := WithContext(ctx, func() error {
		return expected
	})
	if err != expected {
		t.Fail()
	}
}

func TestDoReturnsNil(t *testing.T) {
	ctx := context.Background()
	err := WithContext(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Fail()
	}
}
