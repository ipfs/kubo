package commands

import (
	"testing"
	"sort"
)

func TestShuffleEmpty(t *testing.T) {
	actual := shuffle(0, 3)
	assertLen(t, actual, 0)
}

func TestShuffleSingle(t *testing.T) {
	actual := shuffle(1, 3)
	assertLen(t, actual, 1)
	if actual[0] != 0 {
		t.Errorf("Expected 0 as the only value, but got %d", actual[0])
	}
}

func TestShuffleUnique(t *testing.T) {
	actual := shuffle(100, 100)
	assertLen(t, actual, 100)
	sort.Ints(actual)
	for i, v := range actual {
		if i != v {
			t.Fatalf("Not each value is present exactly once")
		}
	}
}

func assertLen(t *testing.T, actual []int, expected int) {
	actualLen := len(actual)
	if actualLen != expected {
		t.Error("Expected a slice with %d items, but got %d", expected, actualLen)
	}
}
