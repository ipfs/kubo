package util

import (
	"errors"
	"testing"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

func TestLogErrorDoesNotBlockWhenCtxIsNotSetUpForLogging(t *testing.T) {
	ctx := context.Background()
	LogError(ctx, errors.New("ignore me"))
}

func TestLogErrorReceivedByParent(t *testing.T) {

	expected := errors.New("From child to parent")

	ctx, errs := ContextWithErrorLog(context.Background())

	go func() {
		LogError(ctx, expected)
	}()

	if err := <-errs; err != expected {
		t.Fatal("didn't receive the expected error")
	}
}
