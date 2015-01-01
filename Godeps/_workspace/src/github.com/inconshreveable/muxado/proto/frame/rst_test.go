package frame

import (
	"reflect"
	"testing"
)

type RstTestParams struct {
	streamId  StreamId
	errorCode ErrorCode
}

func TestSerializeRst(t *testing.T) {
	t.Parallel()

	cases := []struct {
		params   RstTestParams
		expected []byte
	}{
		{
			RstTestParams{0x49a1bb00, ProtocolError},
			[]byte{0x0, 0x4, 0x0, TypeStreamRst, 0x49, 0xa1, 0xbb, 0x00, 0x0, 0x0, 0x0, ProtocolError},
		},
		{
			RstTestParams{0x0, FlowControlError},
			[]byte{0x0, 0x4, 0x0, TypeStreamRst, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, FlowControlError},
		},
		{
			RstTestParams{streamMask, RefusedStream},
			[]byte{0x00, 0x4, 0x0, TypeStreamRst, 0x7F, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x0, RefusedStream},
		},
	}

	for _, tcase := range cases {
		buf, trans := loadedTrans([]byte{})
		var f *WStreamRst = NewWStreamRst()

		if err := f.Set(tcase.params.streamId, tcase.params.errorCode); err != nil {
			t.Fatalf("Error while setting params %v!", tcase.params)
		}

		if err := f.writeTo(trans); err != nil {
			t.Fatalf("Error while writing %v!", tcase.params)
		}

		if !reflect.DeepEqual(tcase.expected, buf.Bytes()) {
			t.Errorf("Failed to serialize STREAM_RST, expected: %v got %v", tcase.expected, buf.Bytes())
		}
	}
}

func TestDeserializeRst(t *testing.T) {
	t.Parallel()

	_, trans := loadedTrans([]byte{0x00, rstBodySize, 0x0, TypeStreamRst, 0x7F, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x0, RefusedStream})
	h := newHeader()
	if err := h.readFrom(trans); err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}
	var f RStreamRst
	f.Header = h
	if err := f.readFrom(trans); err != nil {
		t.Fatalf("Error while reading rst frame: %v", err)
	}

	if f.ErrorCode() != RefusedStream {
		t.Errorf("Expected error code %d but got %d", RefusedStream, f.ErrorCode())
	}
}

// test a bad frame length of rstBodySize+1
func TestBadLengthRst(t *testing.T) {
	t.Parallel()

	_, trans := loadedTrans([]byte{0x00, rstBodySize + 1, 0x0, TypeStreamRst, 0x7F, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x0, 0x0})
	h := newHeader()
	if err := h.readFrom(trans); err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}
	var f RStreamRst
	f.Header = h
	if err := f.readFrom(trans); err == nil {
		t.Errorf("Expected error when setting bad rst frame length, got none.")
	}
}

// test fewer than rstBodySize bytes available after header
func TestShortReadRst(t *testing.T) {
	t.Parallel()

	_, trans := loadedTrans([]byte{0x00, rstBodySize, 0x0, TypeStreamRst, 0x7F, 0xFF, 0xFF, 0xFF, 0x1})
	h := newHeader()
	if err := h.readFrom(trans); err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}
	var f RStreamRst
	f.Header = h
	if err := f.readFrom(trans); err == nil {
		t.Errorf("Expected error when reading incomplete frame, got none.")
	}
}
