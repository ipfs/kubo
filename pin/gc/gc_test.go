package gc

import (
	"testing"

	"github.com/ipfs/go-ipfs/pin"
)

func TestPinSources(t *testing.T) {
	gc := &gctype{}
	sources := []pin.Source{
		pin.Source{Direct: true},
		pin.Source{Internal: true},
		pin.Source{Strict: true},
		pin.Source{},
	}
	err := gc.AddPinSource(sources...)
	if err != nil {
		t.Fatal(err)
	}

	p := gc.roots[0]
	if !p.Strict {
		t.Errorf("first root should be strict, was %v", p)
	}
	p = gc.roots[1]
	if p.Strict || p.Direct || p.Internal {
		t.Errorf("second root should be normal, was %v", p)
	}
	p = gc.roots[2]
	if !p.Direct {
		t.Errorf("third root should be direct, was %v", p)
	}
	p = gc.roots[3]
	if !p.Internal {
		t.Errorf("fourth root should be internal, was %v", p)
	}

}
