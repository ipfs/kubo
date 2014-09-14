package context

import (
	"errors"
	"sync"
	"testing"
	"time"

	goctx "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func TestChildLogsErrorThenParentCancels(t *testing.T) {
	// This tests two behaviors:
	// 1. the errorReporter can send errors all the way back up the tree.
	// 2. the errorReporter receives a cancellation signal when a middleman
	// sends a cancellation signal
	// TODO(brian): split this into two separate tests
	errorReceivingCtx, errs := WithErrorLog(Background())
	middlemanA, cancelFunc := WithCancel(errorReceivingCtx)
	middlemanB, _ := WithCancel(middlemanA)
	middlemanC, _ := goctx.WithCancel(middlemanB) // the original go.net.context
	errorReporter, _ := WithCancel(middlemanC)

	expected := errors.New("err from errorReporter")
	var goroutines sync.WaitGroup
	goroutines.Add(1)
	go func() {
		errorReporter.LogError(expected) // 0)
		<-errorReporter.Done()           // 3) wait for cancelFunc()
		goroutines.Done()                // 4)
	}()

	received := <-errs // 1) ensure received errorReporter's err
	cancelFunc()       // 2)
	goroutines.Wait()  // 5) ensure child received cancellation signal

	if received.Error() != expected.Error() {
		t.Fail()
	}
}

func TestErrsDoNotLeakUpTree(t *testing.T) {
	alpha, a := WithErrorLog(Background())
	beta, b := WithErrorLog(alpha)
	delta, d := WithErrorLog(beta)
	omega, expectedChan := WithErrorLog(delta)

	expectedErr := errors.New("err from omega ctx")
	go func() {
		omega.LogError(expectedErr)
	}()

	select {
	case <-a:
		t.Fail()
	case <-b:
		t.Fail()
	case <-d:
		t.Fail()
	case received := <-expectedChan:
		if received.Error() != expectedErr.Error() {
			t.Fail()
		}
	}
}

func TestChildWithErrorLogCancelsWhenParentTimesOut(t *testing.T) {
	parent, _ := WithTimeout(Background(), time.Nanosecond)
	if !errorLoggingChildCancelsWhenParentCancels(parent) {
		t.Fail()
	}
}

func TestDeadline(t *testing.T) {
	parent, _ := WithDeadline(Background(), time.Now())
	if !errorLoggingChildCancelsWhenParentCancels(parent) {
		t.Fail()
	}
}

func TestChildGetsValuesFromParentContext(t *testing.T) {
	k := "foo"
	v := "bar"
	parent := WithValue(Background(), k, v)
	if parent.Value(k) != v {
		t.Fail()
	}
	child, _ := WithErrorLog(parent)
	if child.Value(k) != v {
		t.Fail()
	}
}

func TestInheritFromTodoContext(t *testing.T) {
	todoCtx := TODO()
	cancellableCtx, cancelFunc := WithCancel(todoCtx)
	cancelFunc()
	if !errorLoggingChildCancelsWhenParentCancels(cancellableCtx) {
		t.Fail()
	}

}

func errorLoggingChildCancelsWhenParentCancels(parent Context) bool {
	ctx, errs := WithErrorLog(parent)
	select {
	case <-ctx.Done():
		return true
	case <-errs:
	}
	return false
}
