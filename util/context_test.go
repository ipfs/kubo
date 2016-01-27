package util

import (
	"errors"
	"testing"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
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
