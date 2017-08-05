// Strategy PRQ Tests
// ==================

package decision

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	u "gx/ipfs/QmPsAfmDBnZN3kZGSuNwvCNDZiHneERSKmRcFyG3UkvcT3/go-ipfs-util"
	peer "gx/ipfs/QmWNY7dV54ZDYmTA1ykVdwNCqC11mpU4zSUp6XDpLTH9eG/go-libp2p-peer"
	"gx/ipfs/QmeDA8gNhvRTsbrjEieay5wezupJDiky8xvCzDABbsGzmp/go-testutil"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

// Single-Peer Tests
// -----------------

func TestSPRQPushPopLegacy(t *testing.T) {
	prq := newStrategyPRQ(Simple)
	partner := testutil.RandPeerIDFatal(t)
	alphabet := strings.Split("abcdefghijklmnopqrstuvwxyz", "")
	vowels := strings.Split("aeiou", "")
	consonants := func() []string {
		var out []string
		for _, letter := range alphabet {
			skip := false
			for _, vowel := range vowels {
				if letter == vowel {
					skip = true
				}
			}
			if !skip {
				out = append(out, letter)
			}
		}
		return out
	}()
	sort.Strings(alphabet)
	sort.Strings(vowels)
	sort.Strings(consonants)

	// add a bunch of blocks. cancel some. drain the queue. the queue should only have the kept entries

	l := newLedger(partner).Receipt()
	l.Value = 1
	for _, index := range rand.Perm(len(alphabet)) { // add blocks for all letters
		letter := alphabet[index]
		c := cid.NewCidV0(u.Hash([]byte(letter)))
		prq.Push(&wantlist.Entry{Cid: c, Priority: math.MaxInt32 - index}, l)
	}
	for _, consonant := range consonants {
		c := cid.NewCidV0(u.Hash([]byte(consonant)))
		prq.Remove(c, partner)
	}

	var out []string
	for {
		received := prq.Pop()
		if received == nil {
			break
		}
		out = append(out, received.Entry.Cid.String())
	}

	// Entries popped should already be in correct order
	for i, expected := range vowels {
		exp := cid.NewCidV0(u.Hash([]byte(expected))).String()
		if out[i] != exp {
			t.Fatal("received", out[i], "expected", exp)
		}
	}
}

func TestSPRQPushPopServeAll(t *testing.T) {
	roundBurst := 100
	prq := newStrategyPRQCustom(Simple, roundBurst)
	partner := testutil.RandPeerIDFatal(t)
	alphabet := strings.Split("abcdefghijklmnopqrstuvwxyz", "")

	l := newLedger(partner).Receipt()
	l.Value = 1
	size := 5
	for index, letter := range alphabet { // add blocks for all letters
		c := cid.NewCidV0(u.Hash([]byte(letter)))
		prq.Push(&wantlist.Entry{Cid: c, Priority: math.MaxInt32 - index, Size: size}, l)
	}

	expectedAllocation := roundBurst
	var out []string
	for {
		received := prq.Pop()
		if received == nil {
			break
		}
		if expectedAllocation == 0 {
			expectedAllocation = roundBurst
		}
		expectedAllocation -= size
		if prq.allocationForPeer(partner) != expectedAllocation {
			t.Fatalf("Expected allocation of %d, got %d", expectedAllocation, prq.allocationForPeer(partner))
		}
		out = append(out, received.Entry.Cid.String())
	}

	if expectedAllocation != 70 {
		t.Fatalf("Peer should have ended with 70 allocation, but had %d", expectedAllocation)
	}
	if len(out) != len(alphabet) {
		t.Fatalf("Expected %d blocks popped, got %d", len(alphabet), len(out))
	}
	for i, expected := range alphabet {
		exp := cid.NewCidV0(u.Hash([]byte(expected))).String()
		if out[i] != exp {
			t.Fatalf("Expected %s, received %s", exp, out[i])
		}
	}
}

func TestSPRQPushPop1Round(t *testing.T) {
	prq := newStrategyPRQCustom(Simple, 100)
	partner := testutil.RandPeerIDFatal(t)
	alphabet := strings.Split("abcdefghijklmnopqrstuvwxyz", "")
	// the first 20 letters should be served by the end
	expectedOut := strings.Split("abcdefghijklmnopqrst", "")
	expectedRemaining := strings.Split("uvwxyz", "")

	l := newLedger(partner).Receipt()
	l.Value = 1
	size := 5
	for index, letter := range alphabet { // add blocks for all letters
		c := cid.NewCidV0(u.Hash([]byte(letter)))
		prq.Push(&wantlist.Entry{Cid: c, Priority: math.MaxInt32 - index, Size: size}, l)
	}

	expectedAllocation := 100
	var out []string
	firstRound := true
	for {
		if !firstRound && prq.allocationForPeer(partner) == 0 {
			break
		}
		received := prq.Pop()
		firstRound = false
		expectedAllocation -= size
		if prq.allocationForPeer(partner) != expectedAllocation {
			t.Fatalf("Expected allocation of %d, got %d", expectedAllocation, prq.allocationForPeer(partner))
		}
		out = append(out, received.Entry.Cid.String())
	}

	if prq.allocationForPeer(partner) != 0 {
		t.Fatalf("Peer should have 0 allocation, but has %d", prq.allocationForPeer(partner))
	}
	if len(out) != len(expectedOut) {
		t.Fatalf("Expected %d blocks popped, got %d", len(expectedOut), len(out))
	}
	for i, expected := range expectedOut {
		exp := cid.NewCidV0(u.Hash([]byte(expected))).String()
		if out[i] != exp {
			t.Fatalf("Expected %s, received %s", exp, out[i])
		}
	}
	if prq.partners[partner].taskQueue.Len() != len(expectedRemaining) {
		t.Fatalf("Expected %d blocks popped, got %d", len(expectedOut), len(out))
	}
	for _, expected := range expectedRemaining {
		cid := cid.NewCidV0(u.Hash([]byte(expected)))
		if _, ok := prq.taskMap[taskKey(partner, cid)]; !ok {
			t.Fatalf("CID %s was not found in the peer's task map", cid)
		}
	}
}

// Multi-Peer Tests
// ----------------

func TestSPRQPushPop5Peers(t *testing.T) {
	roundBurst := 150
	prq := newStrategyPRQCustom(Simple, roundBurst)
	partners := make([]peer.ID, 5)
	expectedAllocations := make(map[peer.ID]int)
	for i, _ := range partners {
		partners[i] = testutil.RandPeerIDFatal(t)
		expectedAllocations[partners[i]] = (i + 1) * 10
	}
	inputs := [5]string{"a", "ab", "abc", "abcd", "abcde"}

	size := 10
	for i, letters := range inputs {
		l := newLedger(partners[i]).Receipt()
		l.Value = float64(i + 1)
		for j, letter := range strings.Split(letters, "") {
			c := cid.NewCidV0(u.Hash([]byte(letter)))
			prq.Push(&wantlist.Entry{Cid: c, Priority: math.MaxInt32 - j, Size: size}, l)
		}
	}

	numServes := 0
	var out []string
	for {
		received := prq.Pop()
		if received == nil {
			break
		}
		numServes += 1
		expectedAllocations[received.Target] -= size
		if prq.allocationForPeer(received.Target) != expectedAllocations[received.Target] {
			t.Fatalf("Peer %d: Expected allocation of %d, got %d", received.Target.String(),
				expectedAllocations[received.Target], prq.allocationForPeer(received.Target))
		}
		out = append(out, received.Entry.Cid.String())
	}

	if numServes != 15 {
		t.Fatalf("Expected 15 serves, got %d", numServes)
	}
}
