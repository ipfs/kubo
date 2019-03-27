package provider

import (
	"context"
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"testing"
)

func assertReceivesAll(cids []cid.Cid, entries <-chan cid.Cid, t *testing.T) {
	expectedCount := len(cids)

	// madness
	atLeast1 := false
	for entry := range entries {
		atLeast1 = true
		for i, c := range cids {
			if c == entry {
				cids = append(cids[:i], cids[i+1:]...)
				break
			}
		}
	}

	if len(cids) > 0 {
		fmt.Println(cids)
		t.Fatalf("%d entries let over, should be 0", len(cids))
	}

	if !atLeast1 {
		t.Fatalf("expected %d entries, received 0", expectedCount)
	}
}

func TestTrack(t *testing.T) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	tracker := NewTracker(ds)

	cs := makeCids(15)
	for _, c := range cs {
		tracker.Track(c)
	}

	ctx := context.Background()
	tracking, err := tracker.Tracking(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertReceivesAll(cs, tracking, t)
}

func TestUntrack(t *testing.T) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	tracker := NewTracker(ds)

	cs := makeCids(15)
	for _, c := range cs {
		tracker.Track(c)
	}

	for _, c := range cs[:5] {
		tracker.Untrack(c)
	}

	ctx := context.Background()
	tracking, err := tracker.Tracking(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertReceivesAll(cs[5:], tracking, t)
}

func TestIsTracking(t *testing.T) {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	tracker := NewTracker(ds)

	cs := makeCids(2)
	tracker.Track(cs[0])

	isTracking0, err := tracker.IsTracking(cs[0])
	if err != nil {
		t.Fatal(err)
	}

	if !isTracking0 {
		t.Fatalf("should be tracking %v, but is not", cs[0])
	}

	isTracking1, err := tracker.IsTracking(cs[1])
	if err != nil {
		t.Fatal(err)
	}

	if isTracking1 {
		t.Fatalf("should not be tracking %v, but is", cs[1])
	}
}
