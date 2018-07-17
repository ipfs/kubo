package mfs

import (
	"context"
	"testing"
	"time"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ci "gx/ipfs/QmcW4FGAt24fdK1jBgWQn3yP4R9ZLyWQqjozv9QK7epRhL/go-testutil/ci"
)

func TestRepublisher(t *testing.T) {
	if ci.IsRunning() {
		t.Skip("dont run timing tests in CI")
	}

	ctx := context.TODO()

	pub := make(chan struct{})

	pf := func(ctx context.Context, c *cid.Cid) error {
		pub <- struct{}{}
		return nil
	}

	tshort := time.Millisecond * 50
	tlong := time.Second / 2

	rp := NewRepublisher(ctx, pf, tshort, tlong)
	go rp.Run()

	rp.Update(nil)

	// should hit short timeout
	select {
	case <-time.After(tshort * 2):
		t.Fatal("publish didnt happen in time")
	case <-pub:
	}

	cctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			rp.Update(nil)
			time.Sleep(time.Millisecond * 10)
			select {
			case <-cctx.Done():
				return
			default:
			}
		}
	}()

	select {
	case <-pub:
		t.Fatal("shouldnt have received publish yet!")
	case <-time.After((tlong * 9) / 10):
	}
	select {
	case <-pub:
	case <-time.After(tlong / 2):
		t.Fatal("waited too long for pub!")
	}

	cancel()

	go func() {
		err := rp.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	// final pub from closing
	<-pub
}
