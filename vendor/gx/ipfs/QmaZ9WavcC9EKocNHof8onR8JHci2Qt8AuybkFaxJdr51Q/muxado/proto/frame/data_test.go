package frame

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"testing"
)

type fakeTrans struct {
	*bytes.Buffer
}

func (c *fakeTrans) Close() error { return nil }

func loadedTrans(p []byte) (*fakeTrans, *BasicTransport) {
	trans := &fakeTrans{bytes.NewBuffer(p)}
	return trans, NewBasicTransport(trans)
}

type DataTestParams struct {
	streamId StreamId
	data     []byte
	fin      bool
}

func TestSerializeData(t *testing.T) {
	t.Parallel()

	cases := []struct {
		params   DataTestParams
		expected []byte
	}{
		{
			// test a generic data frame
			DataTestParams{0x49a1bb00, []byte{0x00, 0x01, 0x02, 0x03, 0x04}, false},
			[]byte{0x0, 0x5, 0x0, TypeStreamData, 0x49, 0xa1, 0xbb, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04},
		},
		{
			// test a a frame with fin
			DataTestParams{streamMask, []byte{0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00}, true},
			[]byte{0x00, 0x10, flagFin, TypeStreamData, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00},
		},
		{
			// test a zero-length frame
			DataTestParams{0x0, []byte{}, false},
			[]byte{0x0, 0x0, 0x0, TypeStreamData, 0x0, 0x0, 0x0, 0x0},
		},
	}

	for _, tcase := range cases {
		buf, trans := loadedTrans([]byte{})
		var f *WStreamData = NewWStreamData()
		if err := f.Set(tcase.params.streamId, tcase.params.data, tcase.params.fin); err != nil {
			t.Fatalf("Error while setting params %v!", tcase.params)
		}

		if err := f.writeTo(trans); err != nil {
			t.Fatalf("Error while writing %v!", tcase.params)
		}

		if !reflect.DeepEqual(tcase.expected, buf.Bytes()) {
			t.Errorf("Failed to serialize STREAM_DATA, expected: %v got %v", tcase.expected, buf.Bytes())
		}
	}
}

func TestDeserializeData(t *testing.T) {
	_, trans := loadedTrans([]byte{0x00, 0x10, flagFin, TypeStreamData, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00})

	h := newHeader()
	if err := h.readFrom(trans); err != nil {
		t.Fatalf("Failed to read header")
	}

	var f RStreamData
	f.Header = h
	if err := f.readFrom(trans); err != nil {
		t.Fatalf("Read failed with %v", err)
	}

	got, err := ioutil.ReadAll(f.Reader())
	if err != nil {
		t.Fatalf("Error %v while reading data", err)
	}

	expected := []byte{0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00}
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("Wrong bytes read from transport. Expected %v, got %v", expected, got)
	}

	if !f.Fin() {
		t.Errorf("Fin flag was not deserialized")
	}
}

func TestTooLongSerializeData(t *testing.T) {
	t.Parallel()

	var f *WStreamData = NewWStreamData()
	if err := f.Set(0, make([]byte, lengthMask+1), true); err == nil {
		t.Errorf("Expected error when setting too long buffer, got none.")
	}
}

func TestLengthLimitationData(t *testing.T) {
	dataLen := 0x4
	_, trans := loadedTrans([]byte{0x00, byte(dataLen), 0x0, TypeStreamData, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00})

	h := newHeader()
	if err := h.readFrom(trans); err != nil {
		t.Fatalf("Failed to read header")
	}

	var f RStreamData
	f.Header = h
	if err := f.readFrom(trans); err != nil {
		t.Fatalf("Read failed with %v", err)
	}

	got, err := ioutil.ReadAll(f.Reader())
	if err != nil {
		t.Fatalf("Error %v while reading data", err)
	}

	if len(got) != dataLen {
		t.Errorf("Read with wrong number of bytes, got %d expected %d", len(got), 4)
	}

	expected := []byte{0xFF, 0xEE, 0xDD, 0xCC}
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("Wrong bytes read from transport. Expected %v, got %v", expected, got)
	}
}
