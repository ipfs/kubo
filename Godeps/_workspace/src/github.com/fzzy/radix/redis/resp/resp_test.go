package resp

import (
	"bytes"
	"errors"
	. "testing"

	"github.com/stretchr/testify/assert"
)

func TestRead(t *T) {
	var m *Message
	var err error

	_, err = NewMessage(nil)
	assert.NotNil(t, err)

	_, err = NewMessage([]byte{})
	assert.NotNil(t, err)

	// Simple string
	m, _ = NewMessage([]byte("+ohey\r\n"))
	assert.Equal(t, SimpleStr, m.Type)
	assert.Equal(t, []byte("ohey"), m.val)

	// Empty simple string
	m, _ = NewMessage([]byte("+\r\n"))
	assert.Equal(t, SimpleStr, m.Type)
	assert.Equal(t, []byte(""), m.val.([]byte))

	// Error
	m, _ = NewMessage([]byte("-ohey\r\n"))
	assert.Equal(t, Err, m.Type)
	assert.Equal(t, []byte("ohey"), m.val.([]byte))

	// Empty error
	m, _ = NewMessage([]byte("-\r\n"))
	assert.Equal(t, Err, m.Type)
	assert.Equal(t, []byte(""), m.val.([]byte))

	// Int
	m, _ = NewMessage([]byte(":1024\r\n"))
	assert.Equal(t, Int, m.Type)
	assert.Equal(t, int64(1024), m.val.(int64))

	// Bulk string
	m, _ = NewMessage([]byte("$3\r\nfoo\r\n"))
	assert.Equal(t, BulkStr, m.Type)
	assert.Equal(t, []byte("foo"), m.val.([]byte))

	// Empty bulk string
	m, _ = NewMessage([]byte("$0\r\n\r\n"))
	assert.Equal(t, BulkStr, m.Type)
	assert.Equal(t, []byte(""), m.val.([]byte))

	// Nil bulk string
	m, _ = NewMessage([]byte("$-1\r\n"))
	assert.Equal(t, Nil, m.Type)

	// Array
	m, _ = NewMessage([]byte("*2\r\n+foo\r\n+bar\r\n"))
	assert.Equal(t, Array, m.Type)
	assert.Equal(t, 2, len(m.val.([]*Message)))
	assert.Equal(t, SimpleStr, m.val.([]*Message)[0].Type)
	assert.Equal(t, []byte("foo"), m.val.([]*Message)[0].val.([]byte))
	assert.Equal(t, SimpleStr, m.val.([]*Message)[1].Type)
	assert.Equal(t, []byte("bar"), m.val.([]*Message)[1].val.([]byte))

	// Empty array
	m, _ = NewMessage([]byte("*0\r\n"))
	assert.Equal(t, Array, m.Type)
	assert.Equal(t, 0, len(m.val.([]*Message)))

	// Nil Array
	m, _ = NewMessage([]byte("*-1\r\n"))
	assert.Equal(t, Nil, m.Type)

	// Embedded Array
	m, _ = NewMessage([]byte("*3\r\n+foo\r\n+bar\r\n*2\r\n+foo\r\n+bar\r\n"))
	assert.Equal(t, Array, m.Type)
	assert.Equal(t, 3, len(m.val.([]*Message)))
	assert.Equal(t, SimpleStr, m.val.([]*Message)[0].Type)
	assert.Equal(t, []byte("foo"), m.val.([]*Message)[0].val.([]byte))
	assert.Equal(t, SimpleStr, m.val.([]*Message)[1].Type)
	assert.Equal(t, []byte("bar"), m.val.([]*Message)[1].val.([]byte))
	m = m.val.([]*Message)[2]
	assert.Equal(t, 2, len(m.val.([]*Message)))
	assert.Equal(t, SimpleStr, m.val.([]*Message)[0].Type)
	assert.Equal(t, []byte("foo"), m.val.([]*Message)[0].val.([]byte))
	assert.Equal(t, SimpleStr, m.val.([]*Message)[1].Type)
	assert.Equal(t, []byte("bar"), m.val.([]*Message)[1].val.([]byte))

	// Test that two bulks in a row read correctly
	m, _ = NewMessage([]byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"))
	assert.Equal(t, Array, m.Type)
	assert.Equal(t, 2, len(m.val.([]*Message)))
	assert.Equal(t, BulkStr, m.val.([]*Message)[0].Type)
	assert.Equal(t, []byte("foo"), m.val.([]*Message)[0].val.([]byte))
	assert.Equal(t, BulkStr, m.val.([]*Message)[1].Type)
	assert.Equal(t, []byte("bar"), m.val.([]*Message)[1].val.([]byte))
}

type arbitraryTest struct {
	val    interface{}
	expect []byte
}

var nilMessage, _ = NewMessage([]byte("$-1\r\n"))

var arbitraryTests = []arbitraryTest{
	{[]byte("OHAI"), []byte("$4\r\nOHAI\r\n")},
	{"OHAI", []byte("$4\r\nOHAI\r\n")},
	{true, []byte("$1\r\n1\r\n")},
	{false, []byte("$1\r\n0\r\n")},
	{nil, []byte("$-1\r\n")},
	{80, []byte(":80\r\n")},
	{int64(-80), []byte(":-80\r\n")},
	{uint64(80), []byte(":80\r\n")},
	{float32(0.1234), []byte("$6\r\n0.1234\r\n")},
	{float64(0.1234), []byte("$6\r\n0.1234\r\n")},
	{errors.New("hi"), []byte("-hi\r\n")},

	{nilMessage, []byte("$-1\r\n")},

	{[]int{1, 2, 3}, []byte("*3\r\n:1\r\n:2\r\n:3\r\n")},
	{map[int]int{1: 2}, []byte("*2\r\n:1\r\n:2\r\n")},

	{NewSimpleString("OK"), []byte("+OK\r\n")},
}

var arbitraryAsStringTests = []arbitraryTest{
	{[]byte("OHAI"), []byte("$4\r\nOHAI\r\n")},
	{"OHAI", []byte("$4\r\nOHAI\r\n")},
	{true, []byte("$1\r\n1\r\n")},
	{false, []byte("$1\r\n0\r\n")},
	{nil, []byte("$0\r\n\r\n")},
	{80, []byte("$2\r\n80\r\n")},
	{int64(-80), []byte("$3\r\n-80\r\n")},
	{uint64(80), []byte("$2\r\n80\r\n")},
	{float32(0.1234), []byte("$6\r\n0.1234\r\n")},
	{float64(0.1234), []byte("$6\r\n0.1234\r\n")},
	{errors.New("hi"), []byte("$2\r\nhi\r\n")},

	{nilMessage, []byte("$-1\r\n")},

	{[]int{1, 2, 3}, []byte("*3\r\n$1\r\n1\r\n$1\r\n2\r\n$1\r\n3\r\n")},
	{map[int]int{1: 2}, []byte("*2\r\n$1\r\n1\r\n$1\r\n2\r\n")},

	{NewSimpleString("OK"), []byte("+OK\r\n")},
}

var arbitraryAsFlattenedStringsTests = []arbitraryTest{
	{
		[]interface{}{"wat", map[string]interface{}{
			"foo": 1,
		}},
		[]byte("*3\r\n$3\r\nwat\r\n$3\r\nfoo\r\n$1\r\n1\r\n"),
	},
}

func TestWriteArbitrary(t *T) {
	var err error
	buf := bytes.NewBuffer([]byte{})
	for _, test := range arbitraryTests {
		t.Logf("Checking test %v", test)
		buf.Reset()
		err = WriteArbitrary(buf, test.val)
		assert.Nil(t, err)
		assert.Equal(t, test.expect, buf.Bytes())
	}
}

func TestWriteArbitraryAsString(t *T) {
	var err error
	buf := bytes.NewBuffer([]byte{})
	for _, test := range arbitraryAsStringTests {
		t.Logf("Checking test %v", test)
		buf.Reset()
		err = WriteArbitraryAsString(buf, test.val)
		assert.Nil(t, err)
		assert.Equal(t, test.expect, buf.Bytes())
	}
}

func TestWriteArbitraryAsFlattenedStrings(t *T) {
	var err error
	buf := bytes.NewBuffer([]byte{})
	for _, test := range arbitraryAsFlattenedStringsTests {
		t.Logf("Checking test %v", test)
		buf.Reset()
		err = WriteArbitraryAsFlattenedStrings(buf, test.val)
		assert.Nil(t, err)
		assert.Equal(t, test.expect, buf.Bytes())
	}
}

func TestMessageWrite(t *T) {
	var err error
	var m *Message
	buf := bytes.NewBuffer([]byte{})
	for _, test := range arbitraryTests {
		t.Logf("Checking test; %v", test)
		buf.Reset()
		m, err = NewMessage(test.expect)
		assert.Nil(t, err)
		err = WriteMessage(buf, m)
		assert.Nil(t, err)
		assert.Equal(t, test.expect, buf.Bytes())
	}
}
