package path

import (
	"testing"
)

func TestPathParsing(t *testing.T) {
	cases := map[string]bool{
		"/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n":             true,
		"/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a":           true,
		"/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a/b/c/d/e/f": true,
		"/ipns/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a/b/c/d/e/f": true,
		"/ipns/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n":             true,
		"QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a/b/c/d/e/f":       true,
		"QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n":                   true,
		"/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n":                  false,
		"/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a":                false,
		"/ipfs/": false,
		"ipfs/":  false,
		"ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n": false,
	}

	for p, expected := range cases {
		_, err := ParsePath(p)
		valid := (err == nil)
		if valid != expected {
			t.Fatalf("expected %s to have valid == %s", p, expected)
		}
	}
}

func TestIsJustAKey(t *testing.T) {
	cases := map[string]bool{
		"QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n":           true,
		"/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n":     true,
		"/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a":   false,
		"/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a/b": false,
		"/ipns/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n":     false,
	}

	for p, expected := range cases {
		path, err := ParsePath(p)
		if err != nil {
			t.Fatalf("ParsePath failed to parse \"%s\", but should have succeeded", p)
		}
		result := path.IsJustAKey()
		if result != expected {
			t.Fatalf("expected IsJustAKey(%s) to return %v, not %v", p, expected, result)
		}
	}
}
