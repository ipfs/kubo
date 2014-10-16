package bloom

import "testing"

func TestFilter(t *testing.T) {
	f := BasicFilter()
	keys := [][]byte{
		[]byte("hello"),
		[]byte("fish"),
		[]byte("ipfsrocks"),
	}

	f.Add(keys[0])
	if !f.Find(keys[0]) {
		t.Fatal("Failed to find single inserted key!")
	}

	f.Add(keys[1])
	if !f.Find(keys[1]) {
		t.Fatal("Failed to find key!")
	}

	f.Add(keys[2])

	for _, k := range keys {
		if !f.Find(k) {
			t.Fatal("Couldnt find one of three keys")
		}
	}
}
