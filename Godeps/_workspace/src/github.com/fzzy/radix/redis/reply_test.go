package redis

import (
	. "testing"

	"github.com/stretchr/testify/assert"
)

func TestStr(t *T) {
	r := &Reply{Type: ErrorReply, Err: LoadingError}
	_, err := r.Str()
	assert.Equal(t, LoadingError, err)

	r = &Reply{Type: IntegerReply}
	_, err = r.Str()
	assert.NotNil(t, err)

	r = &Reply{Type: StatusReply, buf: []byte("foo")}
	b, err := r.Str()
	assert.Nil(t, err)
	assert.Equal(t, "foo", b)

	r = &Reply{Type: BulkReply, buf: []byte("foo")}
	b, err = r.Str()
	assert.Nil(t, err)
	assert.Equal(t, "foo", b)
}

func TestBytes(t *T) {
	r := &Reply{Type: BulkReply, buf: []byte("foo")}
	b, err := r.Bytes()
	assert.Nil(t, err)
	assert.Equal(t, []byte("foo"), b)
}

func TestInt64(t *T) {
	r := &Reply{Type: ErrorReply, Err: LoadingError}
	_, err := r.Int64()
	assert.Equal(t, LoadingError, err)

	r = &Reply{Type: IntegerReply, int: 5}
	b, err := r.Int64()
	assert.Nil(t, err)
	assert.Equal(t, int64(5), b)

	r = &Reply{Type: BulkReply, buf: []byte("5")}
	b, err = r.Int64()
	assert.Nil(t, err)
	assert.Equal(t, int64(5), b)

	r = &Reply{Type: BulkReply, buf: []byte("foo")}
	_, err = r.Int64()
	assert.NotNil(t, err)
}

func TestInt(t *T) {
	r := &Reply{Type: IntegerReply, int: 5}
	b, err := r.Int()
	assert.Nil(t, err)
	assert.Equal(t, 5, b)
}

func TestBool(t *T) {
	r := &Reply{Type: IntegerReply, int: 0}
	b, err := r.Bool()
	assert.Nil(t, err)
	assert.Equal(t, false, b)

	r = &Reply{Type: StatusReply, buf: []byte("0")}
	b, err = r.Bool()
	assert.Nil(t, err)
	assert.Equal(t, false, b)

	r = &Reply{Type: IntegerReply, int: 2}
	b, err = r.Bool()
	assert.Nil(t, err)
	assert.Equal(t, true, b)

	r = &Reply{Type: NilReply}
	_, err = r.Bool()
	assert.NotNil(t, err)
}

func TestFloat64(t *T) {
	r := &Reply{Type: ErrorReply, Err: LoadingError}
	_, err := r.Float64()
	assert.Equal(t, LoadingError, err)

	r = &Reply{Type: IntegerReply, int: 5}
	_, err = r.Float64()
	assert.NotNil(t, err)

	r = &Reply{Type: BulkReply, buf: []byte("5.1")}
	b, err := r.Float64()
	assert.Nil(t, err)
	assert.Equal(t, float64(5.1), b)

	r = &Reply{Type: BulkReply, buf: []byte("foo")}
	_, err = r.Float64()
	assert.NotNil(t, err)
}

func TestList(t *T) {
	r := &Reply{Type: MultiReply}
	r.Elems = make([]*Reply, 3)
	r.Elems[0] = &Reply{Type: BulkReply, buf: []byte("0")}
	r.Elems[1] = &Reply{Type: NilReply}
	r.Elems[2] = &Reply{Type: BulkReply, buf: []byte("2")}
	l, err := r.List()
	assert.Nil(t, err)
	assert.Equal(t, 3, len(l))
	assert.Equal(t, "0", l[0])
	assert.Equal(t, "", l[1])
	assert.Equal(t, "2", l[2])
}

func TestBytesList(t *T) {
	r := &Reply{Type: MultiReply}
	r.Elems = make([]*Reply, 3)
	r.Elems[0] = &Reply{Type: BulkReply, buf: []byte("0")}
	r.Elems[1] = &Reply{Type: NilReply}
	r.Elems[2] = &Reply{Type: BulkReply, buf: []byte("2")}
	l, err := r.ListBytes()
	assert.Nil(t, err)
	assert.Equal(t, 3, len(l))
	assert.Equal(t, []byte("0"), l[0])
	assert.Nil(t, l[1])
	assert.Equal(t, []byte("2"), l[2])
}

func TestHash(t *T) {
	r := &Reply{Type: MultiReply}
	r.Elems = make([]*Reply, 6)
	r.Elems[0] = &Reply{Type: BulkReply, buf: []byte("a")}
	r.Elems[1] = &Reply{Type: BulkReply, buf: []byte("0")}
	r.Elems[2] = &Reply{Type: BulkReply, buf: []byte("b")}
	r.Elems[3] = &Reply{Type: NilReply}
	r.Elems[4] = &Reply{Type: BulkReply, buf: []byte("c")}
	r.Elems[5] = &Reply{Type: BulkReply, buf: []byte("2")}
	h, err := r.Hash()
	assert.Nil(t, err)
	assert.Equal(t, "0", h["a"])
	assert.Equal(t, "", h["b"])
	assert.Equal(t, "2", h["c"])
}
