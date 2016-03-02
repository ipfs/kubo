package frame

import (
	"reflect"
	"testing"
)

type WndIncTestParams struct {
	streamId StreamId
	inc      uint32
}

func TestSerializeWndInc(t *testing.T) {
	t.Parallel()

	cases := []struct {
		params   WndIncTestParams
		expected []byte
	}{
		{
			WndIncTestParams{0x04b1bd09, 0x0},
			[]byte{0x0, 0x4, 0x0, TypeStreamWndInc, 0x04, 0xb1, 0xbd, 0x09, 0x0, 0x0, 0x0, 0x0},
		},
		{
			WndIncTestParams{0x0, 0x12c498},
			[]byte{0x0, 0x4, 0x0, TypeStreamWndInc, 0x0, 0x0, 0x0, 0x0, 0x0, 0x12, 0xc4, 0x98},
		},
		{
			WndIncTestParams{streamMask, wndIncMask},
			[]byte{0x00, 0x4, 0x0, TypeStreamWndInc, 0x7F, 0xFF, 0xFF, 0xFF, 0x7F, 0xFF, 0xFF, 0xFF},
		},
	}

	for _, tcase := range cases {
		buf, trans := loadedTrans([]byte{})
		var f *WStreamWndInc = NewWStreamWndInc()

		if err := f.Set(tcase.params.streamId, tcase.params.inc); err != nil {
			t.Fatalf("Error while setting params %v!", tcase.params)
		}

		if err := f.writeTo(trans); err != nil {
			t.Fatalf("Error while writing %v!", tcase.params)
		}

		if !reflect.DeepEqual(tcase.expected, buf.Bytes()) {
			t.Errorf("Failed to serialize STREAM_WNDINC, expected: %v got %v", tcase.expected, buf.Bytes())
		}
	}
}

func TestDeserializeWndInc(t *testing.T) {
	t.Parallel()

	_, trans := loadedTrans([]byte{0x00, wndIncBodySize, 0x0, TypeStreamWndInc, 0x7F, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0xc9, 0xF1})
	h := newHeader()
	if err := h.readFrom(trans); err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}
	var f RStreamWndInc
	f.Header = h
	if err := f.readFrom(trans); err != nil {
		t.Fatalf("Error while reading rst frame: %v", err)
	}

	if f.WindowIncrement() != 0xc9f1 {
		t.Errorf("Expected error code %d but got %d", 0xc9f1, f.WindowIncrement())
	}
}

// test a bad frame length of wndIncBodySize+1
func TestBadLengthWndInc(t *testing.T) {
	t.Parallel()

	_, trans := loadedTrans([]byte{0x00, wndIncBodySize + 1, 0x0, TypeStreamWndInc, 0x7F, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x0, 0x0})
	h := newHeader()
	if err := h.readFrom(trans); err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}
	var f RStreamWndInc
	f.Header = h
	if err := f.readFrom(trans); err == nil {
		t.Errorf("Expected error when setting bad wndinc frame length, got none.")
	}
}

// test fewer than rstBodySize bytes available after header
func TestShortReadWndInc(t *testing.T) {
	t.Parallel()

	_, trans := loadedTrans([]byte{0x00, wndIncBodySize, 0x0, TypeStreamWndInc, 0x7F, 0xFF, 0xFF, 0xFF, 0x1})
	h := newHeader()
	if err := h.readFrom(trans); err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}
	var f RStreamWndInc
	f.Header = h
	if err := f.readFrom(trans); err == nil {
		t.Errorf("Expected error when reading incomplete frame, got none.")
	}
}
