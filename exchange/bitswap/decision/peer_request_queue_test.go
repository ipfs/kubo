package decision

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	"github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/testutil"
)

func TestPushPop(t *testing.T) {
	prq := newPRQ()
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

	for _, index := range rand.Perm(len(alphabet)) { // add blocks for all letters
		letter := alphabet[index]
		t.Log(partner.String())
		prq.Push(wantlist.Entry{Key: util.Key(letter), Priority: math.MaxInt32 - index}, partner)
	}
	for _, consonant := range consonants {
		prq.Remove(util.Key(consonant), partner)
	}

	for _, expected := range vowels {
		received := prq.Pop().Entry.Key
		if received != util.Key(expected) {
			t.Fatal("received", string(received), "expected", string(expected))
		}
	}
}
