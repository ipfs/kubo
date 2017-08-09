package blockstore

import (
	"testing"

	"github.com/ipfs/go-ipfs/blocks/blocksutil"
)

func TestBasicEventHandling(t *testing.T) {
	es := newEventSystem()
	blkgen := blocksutil.NewBlockGenerator()
	b := blkgen.Next()
	e := &Event{
		cid:   b.Cid(),
		block: b,
	}

	ok := false
	handler := func(ei *Event) (bool, error) {
		if e != ei {
			t.Fatal("event is different")
		}
		ok = true
		return false, nil
	}

	es.addHandler(EventPostPut, handler)
	es.fireEvent(EventPostPut, e)
	if !ok {
		t.Fatal("event was not fired")
	}
}

func TestEventMarkedDone(t *testing.T) {
	es := newEventSystem()
	blkgen := blocksutil.NewBlockGenerator()
	b := blkgen.Next()
	e := &Event{
		cid:   b.Cid(),
		block: b,
	}

	count := 0
	handler := func(ei *Event) (bool, error) {
		if e != ei {
			t.Fatal("event is different")
		}
		count++
		return true, nil
	}

	es.addHandler(EventPostPut, handler)
	es.addHandler(EventPostPut, handler)
	es.addHandler(EventPostPut, handler)
	es.fireEvent(EventPostPut, e)
	if count != 1 {
		t.Fatalf("expected to fire the handler 1 time, it was fired %d times", count)
	}
}

func TestEventFireMultiple(t *testing.T) {
	es := newEventSystem()
	blkgen := blocksutil.NewBlockGenerator()
	N := 10
	evs := make([]*Event, N)
	for i, _ := range evs {
		b := blkgen.Next()
		evs[i] = &Event{
			cid:   b.Cid(),
			block: b,
		}
	}

	count := 0
	handler := func(ei *Event) (bool, error) {
		if evs[count] != ei {
			t.Fatal("event is different")
		}
		count++
		return false, nil
	}

	es.addHandler(EventPostPut, handler)
	es.fireMany(EventPostPut, evs)
	if count != N {
		t.Fatalf("events were not fired, count should be %d, is %d", N, count)
	}
}

func TestEventFireMultipleMarkDone(t *testing.T) {
	es := newEventSystem()
	blkgen := blocksutil.NewBlockGenerator()
	N := 10
	evs := make([]*Event, N)
	for i, _ := range evs {
		b := blkgen.Next()
		evs[i] = &Event{
			cid:   b.Cid(),
			block: b,
		}
	}

	count := 0
	handler := func(ei *Event) (bool, error) {
		if evs[count] != ei {
			t.Fatal("event is different")
		}
		count++
		return true, nil
	}
	failHandler := func(ei *Event) (bool, error) {
		t.Fatal("this should not fire")
		return false, nil
	}

	es.addHandler(EventPostPut, handler)
	es.addHandler(EventPostPut, failHandler)
	es.fireMany(EventPostPut, evs)
	if count != N {
		t.Fatalf("expected to fire the handler 1 time, it was fired %d times", count)
	}
}

func BenchmarkEventFireSingle(b *testing.B) {
	b.N = 1000000
	es := newEventSystem()
	blkgen := blocksutil.NewBlockGenerator()
	evs := make([]*Event, b.N)
	for i, _ := range evs {
		b := blkgen.Next()
		evs[i] = &Event{
			cid:   b.Cid(),
			block: b,
		}
	}
	count := 0
	h := func(ei *Event) (bool, error) {
		count++
		return false, nil
	}
	es.addHandler(EventPostPut, h)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		es.fireEvent(EventPostPut, evs[i])
	}
}

func BenchmarkEventFireMultiple(b *testing.B) {
	b.N = 1000000
	es := newEventSystem()
	blkgen := blocksutil.NewBlockGenerator()
	evs := make([]*Event, b.N)
	for i, _ := range evs {
		b := blkgen.Next()
		evs[i] = &Event{
			cid:   b.Cid(),
			block: b,
		}
	}
	count := 0
	h := func(ei *Event) (bool, error) {
		count++
		return false, nil
	}
	es.addHandler(EventPostPut, h)

	b.ReportAllocs()
	b.ResetTimer()

	es.fireMany(EventPostPut, evs)
}
