package context

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestChildLogsErrorThenParentCancels(t *testing.T) {
	loggingCtx, errs := WithErrorLog(Background())
	child, cancelFunc := WithCancel(loggingCtx)
	grandchild, _ := WithCancel(child)
	greatgrandchild, _ := WithCancel(grandchild)

	expected := errors.New("err from greatgrandchild")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		greatgrandchild.LogError(expected) // 0)
		<-greatgrandchild.Done()           // 3) wait for cancelFunc()
		wg.Done()                          // 4)
	}()

	received := <-errs // 1) ensure received greatgrandchild's err
	cancelFunc()       // 2)
	wg.Wait()          // 5) ensure child received cancellation signal

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

func TestTimeout(t *testing.T) {
	ctx, _ := WithTimeout(Background(), time.Nanosecond)
	<-ctx.Done()
}

func TestDeadline(t *testing.T) {
	ctx, _ := WithDeadline(Background(), time.Now())
	<-ctx.Done()
}

func TestWithValue(t *testing.T) {
	k := "foo"
	v := "bar"
	ctx := WithValue(Background(), k, v)
	if ctx.Value(k) != v {
		t.Fail()
	}
}
