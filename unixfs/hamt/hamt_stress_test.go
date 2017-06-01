package hamt

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"
	ft "github.com/ipfs/go-ipfs/unixfs"
)

func getNames(prefix string, count int) []string {
	out := make([]string, count)
	for i := 0; i < count; i++ {
		out[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return out
}

const (
	opAdd = iota
	opDel
	opFind
)

type testOp struct {
	Op  int
	Val string
}

func stringArrToSet(arr []string) map[string]bool {
	out := make(map[string]bool)
	for _, s := range arr {
		out[s] = true
	}
	return out
}

// generate two different random sets of operations to result in the same
// ending directory (same set of entries at the end) and execute each of them
// in turn, then compare to ensure the output is the same on each.
func TestOrderConsistency(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("using seed = %d", seed)
	ds := mdtest.Mock()

	shardWidth := 1024

	keep := getNames("good", 4000)
	temp := getNames("tempo", 6000)

	ops := genOpSet(seed, keep, temp)
	s, err := executeOpSet(t, ds, shardWidth, ops)
	if err != nil {
		t.Fatal(err)
	}

	err = validateOpSetCompletion(t, s, keep, temp)
	if err != nil {
		t.Fatal(err)
	}

	ops2 := genOpSet(seed+1000, keep, temp)
	s2, err := executeOpSet(t, ds, shardWidth, ops2)
	if err != nil {
		t.Fatal(err)
	}

	err = validateOpSetCompletion(t, s2, keep, temp)
	if err != nil {
		t.Fatal(err)
	}

	nd, err := s.Node()
	if err != nil {
		t.Fatal(err)
	}

	nd2, err := s2.Node()
	if err != nil {
		t.Fatal(err)
	}

	k := nd.Cid()
	k2 := nd2.Cid()

	if !k.Equals(k2) {
		t.Fatal("got different results: ", k, k2)
	}
}

func validateOpSetCompletion(t *testing.T, s *HamtShard, keep, temp []string) error {
	ctx := context.TODO()
	for _, n := range keep {
		_, err := s.Find(ctx, n)
		if err != nil {
			return fmt.Errorf("couldnt find %s: %s", n, err)
		}
	}

	for _, n := range temp {
		_, err := s.Find(ctx, n)
		if err != os.ErrNotExist {
			return fmt.Errorf("expected not to find: %s", err)
		}
	}

	return nil
}

func executeOpSet(t *testing.T, ds dag.DAGService, width int, ops []testOp) (*HamtShard, error) {
	ctx := context.TODO()
	s, err := NewHamtShard(ds, width)
	if err != nil {
		return nil, err
	}

	e := ft.EmptyDirNode()
	ds.Add(e)

	for _, o := range ops {
		switch o.Op {
		case opAdd:
			err := s.Set(ctx, o.Val, e)
			if err != nil {
				return nil, fmt.Errorf("inserting %s: %s", o.Val, err)
			}
		case opDel:
			err := s.Remove(ctx, o.Val)
			if err != nil {
				return nil, fmt.Errorf("deleting %s: %s", o.Val, err)
			}
		case opFind:
			_, err := s.Find(ctx, o.Val)
			if err != nil {
				return nil, fmt.Errorf("finding %s: %s", o.Val, err)
			}
		}
	}

	return s, nil
}

func genOpSet(seed int64, keep, temp []string) []testOp {
	tempset := stringArrToSet(temp)

	allnames := append(keep, temp...)
	shuffle(seed, allnames)

	var todel []string

	var ops []testOp

	for {
		n := len(allnames) + len(todel)
		if n == 0 {
			return ops
		}

		rn := rand.Intn(n)

		if rn < len(allnames) {
			next := allnames[0]
			allnames = allnames[1:]
			ops = append(ops, testOp{
				Op:  opAdd,
				Val: next,
			})

			if tempset[next] {
				todel = append(todel, next)
			}
		} else {
			shuffle(seed+100, todel)
			next := todel[0]
			todel = todel[1:]

			ops = append(ops, testOp{
				Op:  opDel,
				Val: next,
			})
		}
	}
}
