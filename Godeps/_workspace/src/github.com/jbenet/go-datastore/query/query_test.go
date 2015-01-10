package query

import (
	"strings"
	"testing"
)

var sampleKeys = []string{
	"/ab/c",
	"/ab/cd",
	"/a",
	"/abce",
	"/abcf",
	"/ab",
}

type testCase struct {
	keys   []string
	expect []string
}

func testResults(t *testing.T, res Results, expect []string) {
	actualE, err := res.Rest()
	if err != nil {
		t.Fatal(err)
	}

	actual := make([]string, len(actualE))
	for i, e := range actualE {
		actual[i] = e.Key
	}

	if len(actual) != len(expect) {
		t.Error("expect != actual.", expect, actual)
	}

	if strings.Join(actual, "") != strings.Join(expect, "") {
		t.Error("expect != actual.", expect, actual)
	}
}

func TestLimit(t *testing.T) {
	testKeyLimit := func(t *testing.T, limit int, keys []string, expect []string) {
		e := make([]Entry, len(keys))
		for i, k := range keys {
			e[i] = Entry{Key: k}
		}

		res := ResultsWithEntries(Query{}, e)
		res = NaiveLimit(res, limit)
		testResults(t, res, expect)
	}

	testKeyLimit(t, 0, sampleKeys, []string{ // none
		"/ab/c",
		"/ab/cd",
		"/a",
		"/abce",
		"/abcf",
		"/ab",
	})

	testKeyLimit(t, 10, sampleKeys, []string{ // large
		"/ab/c",
		"/ab/cd",
		"/a",
		"/abce",
		"/abcf",
		"/ab",
	})

	testKeyLimit(t, 2, sampleKeys, []string{
		"/ab/c",
		"/ab/cd",
	})
}

func TestOffset(t *testing.T) {

	testOffset := func(t *testing.T, offset int, keys []string, expect []string) {
		e := make([]Entry, len(keys))
		for i, k := range keys {
			e[i] = Entry{Key: k}
		}

		res := ResultsWithEntries(Query{}, e)
		res = NaiveOffset(res, offset)
		testResults(t, res, expect)
	}

	testOffset(t, 0, sampleKeys, []string{ // none
		"/ab/c",
		"/ab/cd",
		"/a",
		"/abce",
		"/abcf",
		"/ab",
	})

	testOffset(t, 10, sampleKeys, []string{ // large
	})

	testOffset(t, 2, sampleKeys, []string{
		"/a",
		"/abce",
		"/abcf",
		"/ab",
	})
}
