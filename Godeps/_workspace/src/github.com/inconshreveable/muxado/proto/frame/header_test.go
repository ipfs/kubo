package frame

import (
	"reflect"
	"testing"
)

type HeaderParams struct {
	ftype    FrameType
	length   int
	streamId StreamId
	flags    flagsType
}

func (params *HeaderParams) checkDeserialize(t *testing.T, h Header) {
	if h.Type() != params.ftype {
		t.Errorf("Failed deserialization. Expected type %x, got: %x", params.ftype, h.Type())
	}

	if h.Length() != uint16(params.length) {
		t.Errorf("Failed deserialization. Expected length %x, got: %x", params.length, h.Length())
	}

	if h.Flags() != params.flags {
		t.Errorf("Failed deserialization. Expected flags %x, got: %x", params.flags, h.Flags())
	}

	if h.StreamId() != params.streamId {
		t.Errorf("Failed deserialization. Expected stream id %x, got: %x", params.streamId, h.StreamId())
	}
}

func TestHeaderSerialization(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input          HeaderParams
		expectedOutput []byte
	}{
		{
			HeaderParams{
				ftype:    TypeStreamRst,
				length:   0x4,
				streamId: 0x2843,
				flags:    0,
			},
			[]byte{0, 0x4, 0, 0x2, 0, 0, 0x28, 0x43},
		},
		{
			HeaderParams{
				ftype:    0x1F,
				length:   0x37BD,
				streamId: 0x0,
				flags:    0x9,
			},
			[]byte{0x37, 0xBD, 0x9, 0x1F, 0, 0, 0, 0},
		},
		{
			HeaderParams{
				ftype:    0,
				length:   0,
				streamId: 0,
				flags:    0,
			},
			[]byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			HeaderParams{
				ftype:    typeMask,
				length:   lengthMask,
				streamId: streamMask,
				flags:    flagsMask,
			},
			[]byte{0x3F, 0xFF, 0xFF, 0x1F, 0x7F, 0xFF, 0xFF, 0xFF},
		},
		{
			HeaderParams{
				ftype:    0x1e,
				length:   0x1DAA,
				streamId: 0x4F224719,
				flags:    0x17,
			},
			[]byte{0x1D, 0xAA, 0x17, 0x1E, 0x4F, 0x22, 0x47, 0x19},
		},
	}

	for _, test := range testCases {
		var h Header = Header(make([]byte, headerSize))
		h.SetAll(test.input.ftype, test.input.length, test.input.streamId, test.input.flags)
		output := []byte(h)
		if !reflect.DeepEqual(output, test.expectedOutput) {
			t.Errorf("Failed serialization of %v. Expected %x, got: %x", test.input, output, test.expectedOutput)
		}
	}
}

func TestHeaderDeserialization(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input          []byte
		expectedOutput HeaderParams
	}{
		{
			[]byte{0, 0x4, 0, 0x2, 0, 0, 0x28, 0x43},
			HeaderParams{
				ftype:    TypeStreamRst,
				length:   0x4,
				streamId: 0x2843,
				flags:    0,
			},
		},
		{
			[]byte{0x37, 0xBD, 0x9, 0x1F, 0, 0, 0, 0},
			HeaderParams{
				ftype:    0x1F,
				length:   0x37BD,
				streamId: 0x0,
				flags:    0x9,
			},
		},
		{
			[]byte{0, 0, 0, 0, 0, 0, 0, 0},
			HeaderParams{
				ftype:    0,
				length:   0,
				streamId: 0,
				flags:    0,
			},
		},
		{
			[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			HeaderParams{
				ftype:    typeMask,
				length:   lengthMask,
				streamId: streamMask,
				flags:    flagsMask,
			},
		},
		{
			[]byte{0x9D, 0xAA, 0x17, 0xF0, 0xCF, 0x22, 0x47, 0x19},
			HeaderParams{
				ftype:    0x10,
				length:   0x1DAA,
				streamId: 0x4F224719,
				flags:    0x17,
			},
		},
	}

	for _, test := range testCases {
		test.expectedOutput.checkDeserialize(t, Header(test.input))

	}
}

func TestHeaderRoundTrip(t *testing.T) {
	t.Parallel()

	headers := []HeaderParams{
		HeaderParams{
			ftype:    TypeStreamRst,
			length:   0x4,
			streamId: 0x2843,
			flags:    0,
		},
		HeaderParams{
			ftype:    0x1F,
			length:   0x37BD,
			streamId: 0x0,
			flags:    0x9,
		},
		HeaderParams{
			ftype:    0,
			length:   0,
			streamId: 0,
			flags:    0,
		},
		HeaderParams{
			ftype:    typeMask,
			length:   lengthMask,
			streamId: streamMask,
			flags:    flagsMask,
		},
		HeaderParams{
			ftype:    0x1e,
			length:   0x1DAA,
			streamId: 0x4F224719,
			flags:    0x17,
		},
	}

	for _, input := range headers {
		var h Header = Header(make([]byte, headerSize))
		h.SetAll(input.ftype, input.length, input.streamId, input.flags)
		input.checkDeserialize(t, h)
	}
}

func TestValidStreamIds(t *testing.T) {
	t.Parallel()

	validStreamIds := []StreamId{
		0x0,
		0xFF,
		0x23C10A8F,
		0x7FFFFFFF,
	}

	for _, validStreamId := range validStreamIds {
		var h Header = Header(make([]byte, headerSize))
		err := h.SetAll(TypeStreamSyn, 0, validStreamId, 0)
		if err != nil {
			t.Errorf("Failed to create frame header with valid stream id %d.", validStreamId)
		}

	}
}

func TestInvalidStreamId(t *testing.T) {
	t.Parallel()

	invalidStreamIds := []StreamId{
		0xF0000000,
		0xB012CA8E,
		0x80000000,
		0xFFFFFFFF,
	}

	for _, invalidStreamId := range invalidStreamIds {
		var h Header = Header(make([]byte, headerSize))
		err := h.SetAll(TypeStreamSyn, 0, invalidStreamId, 0)
		if err == nil {
			t.Errorf("Failed to error on invalid stream id %d.", invalidStreamId)
		}

	}
}

func TestValidLengths(t *testing.T) {
	t.Parallel()

	validLengths := []int{
		0x0,
		0x2FF,
		0x301A,
		0x3FFF,
	}

	for _, validLength := range validLengths {
		var h Header = Header(make([]byte, headerSize))
		err := h.SetAll(TypeStreamSyn, validLength, 0, 0)
		if err != nil {
			t.Errorf("Failed to create frame header with valid length %d.", validLength)
		}

	}
}

func TestInvalidLengths(t *testing.T) {
	t.Parallel()

	invalidLengths := []int{
		-1,
		0x4000,
		0xB012,
		0x8000,
		0xFFFF,
	}

	for _, invalidLength := range invalidLengths {
		var h Header = Header(make([]byte, headerSize))
		err := h.SetAll(TypeStreamSyn, invalidLength, 0, 0)
		if err == nil {
			t.Errorf("Failed to error on invalid length %d.", invalidLength)
		}

	}
}
