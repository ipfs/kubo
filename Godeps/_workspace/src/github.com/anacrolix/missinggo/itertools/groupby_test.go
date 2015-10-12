package itertools

import (
	"testing"

	"github.com/stretchr/testify/require"
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
