package service

import (
	"bytes"
	"testing"
)

func TestMarshaling(t *testing.T) {

	test := func(d1 []byte, rid1 RequestID) {
		d2, err := wrapData(d1, rid1)
		if err != nil {
			t.Error(err)
		}

		d3, rid2, err := unwrapData(d2)
		if err != nil {
			t.Error(err)
		}

		d4, err := wrapData(d3, rid1)
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(rid2, rid1) {
			t.Error("RequestID fail")
		}

		if !bytes.Equal(d1, d3) {
			t.Error("unmarshalled data should be the same")
		}

		if !bytes.Equal(d2, d4) {
			t.Error("marshalled data should be the same")
		}
	}

	test([]byte("foo"), []byte{1, 2, 3, 4})
	test([]byte("bar"), nil)
}
