package util

import (
	"errors"
	"testing"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

func TestDoReturnsContextErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan struct{})
	err := ContextDo(ctx, func() error {
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
	err := ContextDo(ctx, func() error {
		return expected
	})
	if err != expected {
		t.Fail()
	}
}

func TestDoReturnsNil(t *testing.T) {
	ctx := context.Background()
	err := ContextDo(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Fail()
	}
}
