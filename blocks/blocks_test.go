package blocks

import "testing"

func TestBlocksBasic(t *testing.T) {

	// Test empty data
	empty := []byte{}
	_, err := NewBlock(empty)
	if err != nil {
		t.Fatal(err)
	}

	// Test nil case
	_, err = NewBlock(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Test some data
	_, err = NewBlock([]byte("Hello world!"))
	if err != nil {
		t.Fatal(err)
	}
}
