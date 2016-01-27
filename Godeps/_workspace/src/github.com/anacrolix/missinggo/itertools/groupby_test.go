package itertools

import (
	"testing"

	"gx/ipfs/QmZwjfAKWe7vWZ8f48u7AGA1xYfzR1iCD9A2XSCYFRBWot/testify/require"
)

func TestGroupByKey(t *testing.T) {
	var ks []byte
	gb := GroupBy(StringIterator("AAAABBBCCDAABBB"), nil)
	for gb.Next() {
		ks = append(ks, gb.Value().(Group).Key().(byte))
	}
	t.Log(ks)
	require.EqualValues(t, "ABCDAB", ks)
}

func TestGroupByList(t *testing.T) {
	var gs []string
	gb := GroupBy(StringIterator("AAAABBBCCD"), nil)
	for gb.Next() {
		i := gb.Value().(Iterator)
		var g string
		for i.Next() {
			g += string(i.Value().(byte))
		}
		gs = append(gs, g)
	}
	t.Log(gs)
}
