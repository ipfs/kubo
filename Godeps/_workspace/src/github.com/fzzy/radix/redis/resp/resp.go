// This package provides an easy to use interface for creating and parsing
// messages encoded in the REdis Serialization Protocol (RESP). You can check
// out more details about the protocol here: http://redis.io/topics/protocol
package resp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

var (
	delim    = []byte{'\r', '\n'}
	delimEnd = delim[len(delim)-1]
)

type Type int

const (
	SimpleStr Type = iota
	Err
	Int
	BulkStr
	Array
	Nil
)

var (
	simpleStrPrefix = []byte{'+'}
	errPrefix       = []byte{'-'}
	intPrefix       = []byte{':'}
	bulkStrPrefix   = []byte{'$'}
	arrayPrefix     = []byte{'*'}
)

// Parse errors
var (
	badType  = errors.New("wrong type")
	parseErr = errors.New("parse error")
)

type Message struct {
	Type
	val interface{}
	raw []byte
}

// NewMessagePParses the given raw message and returns a Message struct
// representing it
func NewMessage(b []byte) (*Message, error) {
	return ReadMessage(bytes.NewReader(b))
}

// Can be used when writing to a resp stream to write a simple-string-style
// stream (e.g. +OK\r\n) instead of the default bulk-string-style strings.
//
// 	foo := NewSimpleString("foo")
// 	bar := NewSimpleString("bar")
// 	baz := NewSimpleString("baz")
// 	resp.WriteArbitrary(w, foo)
// 	resp.WriteArbitrary(w, []interface{}{bar, baz})
//
func NewSimpleString(s string) *Message {
	b := append(make([]byte, 0, len(s)+3), '+')
	b = append(b, []byte(s)...)
	b = append(b, '\r', '\n')
	return &Message{
		Type: SimpleStr,
		val:  s,
		raw:  b,
	}
}

// ReadMessage attempts to read a message object from the given io.Reader, parse
// it, and return a Message struct representing it
func ReadMessage(reader io.Reader) (*Message, error) {
	r := bufio.NewReader(reader)
	return bufioReadMessage(r)
}

func bufioReadMessage(r *bufio.Reader) (*Message, error) {
	b, err := r.Peek(1)
	if err != nil {
		return nil, err
	}
	switch b[0] {
	case simpleStrPrefix[0]:
		return readSimpleStr(r)
	case errPrefix[0]:
		return readError(r)
	case intPrefix[0]:
		return readInt(r)
	case bulkStrPrefix[0]:
		return readBulkStr(r)
	case arrayPrefix[0]:
		return readArray(r)
	default:
		return nil, badType
	}
}

func readSimpleStr(r *bufio.Reader) (*Message, error) {
	b, err := r.ReadBytes(delimEnd)
	if err != nil {
		return nil, err
	}
	return &Message{Type: SimpleStr, val: b[1 : len(b)-2], raw: b}, nil
}

func readError(r *bufio.Reader) (*Message, error) {
	b, err := r.ReadBytes(delimEnd)
	if err != nil {
		return nil, err
	}
	return &Message{Type: Err, val: b[1 : len(b)-2], raw: b}, nil
}

func readInt(r *bufio.Reader) (*Message, error) {
	b, err := r.ReadBytes(delimEnd)
	if err != nil {
		return nil, err
	}
	i, err := strconv.ParseInt(string(b[1:len(b)-2]), 10, 64)
	if err != nil {
		return nil, parseErr
	}
	return &Message{Type: Int, val: i, raw: b}, nil
}

func readBulkStr(r *bufio.Reader) (*Message, error) {
	b, err := r.ReadBytes(delimEnd)
	if err != nil {
		return nil, err
	}
	size, err := strconv.ParseInt(string(b[1:len(b)-2]), 10, 64)
	if err != nil {
		return nil, parseErr
	}
	if size < 0 {
		return &Message{Type: Nil, raw: b}, nil
	}
	total := make([]byte, size)
	b2 := total
	var n int
	for len(b2) > 0 {
		n, err = r.Read(b2)
		if err != nil {
			return nil, err
		}
		b2 = b2[n:]
	}

	// There's a hanging \r\n there, gotta read past it
	trail := make([]byte, 2)
	for i := 0; i < 2; i++ {
		if c, err := r.ReadByte(); err != nil {
			return nil, err
		} else {
			trail[i] = c
		}
	}

	blens := len(b) + len(total)
	raw := make([]byte, 0, blens+2)
	raw = append(raw, b...)
	raw = append(raw, total...)
	raw = append(raw, trail...)
	return &Message{Type: BulkStr, val: total, raw: raw}, nil
}

func readArray(r *bufio.Reader) (*Message, error) {
	b, err := r.ReadBytes(delimEnd)
	if err != nil {
		return nil, err
	}
	size, err := strconv.ParseInt(string(b[1:len(b)-2]), 10, 64)
	if err != nil {
		return nil, parseErr
	}
	if size < 0 {
		return &Message{Type: Nil, raw: b}, nil
	}

	arr := make([]*Message, size)
	for i := range arr {
		m, err := bufioReadMessage(r)
		if err != nil {
			return nil, err
		}
		arr[i] = m
		b = append(b, m.raw...)
	}
	return &Message{Type: Array, val: arr, raw: b}, nil
}

// Bytes returns a byte slice representing the value of the Message. Only valid
// for a Message of type SimpleStr, Err, and BulkStr. Others will return an
// error
func (m *Message) Bytes() ([]byte, error) {
	if b, ok := m.val.([]byte); ok {
		return b, nil
	}
	return nil, badType
}

// Str is a Convenience method around Bytes which converts the output to a
// string
func (m *Message) Str() (string, error) {
	b, err := m.Bytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Int returns an int64 representing the value of the Message. Only valid for
// Int messages
func (m *Message) Int() (int64, error) {
	if i, ok := m.val.(int64); ok {
		return i, nil
	}
	return 0, badType
}

// Err returns an error representing the value of the Message. Only valid for
// Err messages
func (m *Message) Err() (error, error) {
	if m.Type != Err {
		return nil, badType
	}
	s, err := m.Str()
	if err != nil {
		return nil, err
	}
	return errors.New(s), nil
}

// Array returns the Message slice encompassed by this Messsage, assuming the
// Message is of type Array
func (m *Message) Array() ([]*Message, error) {
	if a, ok := m.val.([]*Message); ok {
		return a, nil
	}
	return nil, badType
}

func writeBytesHelper(w io.Writer, b []byte, lastErr error) error {
	if lastErr != nil {
		return lastErr
	}
	_, err := w.Write(b)
	return err
}

// WriteMessage takes in the given Message and writes its encoded form to the
// given io.Writer
func WriteMessage(w io.Writer, m *Message) error {
	_, err := w.Write(m.raw)
	return err
}

// AppendArbitrary takes in any primitive golang value, or Message, and appends
// its encoded form to the given buffer, inferring types where appropriate. It
// then returns the appended buffer
func AppendArbitrary(buf []byte, m interface{}) []byte {
	return appendArb(buf, m, false, false)
}

// WriteArbitrary takes in any primitive golang value, or Message, and writes
// its encoded form to the given io.Writer, inferring types where appropriate.
func WriteArbitrary(w io.Writer, m interface{}) error {
	buf := AppendArbitrary(make([]byte, 0, 1024), m)
	_, err := w.Write(buf)
	return err
}

// AppendArbitraryAsString is similar to AppendArbitraryAsFlattenedString except
// that it won't flatten any embedded arrays.
func AppendArbitraryAsStrings(buf []byte, m interface{}) []byte {
	return appendArb(buf, m, true, false)
}

// WriteArbitraryAsString is similar to WriteArbitraryAsFlattenedString except
// that it won't flatten any embedded arrays.
func WriteArbitraryAsString(w io.Writer, m interface{}) error {
	buf := AppendArbitraryAsStrings(make([]byte, 0, 1024), m)
	_, err := w.Write(buf)
	return err
}

// AppendArbitraryAsFlattenedStrings is similar to AppendArbitrary except that
// it will encode all types except Array as a BulkStr, converting the argument
// into a string first as necessary. It will also flatten any embedded arrays
// into a single long array. This is useful because commands to a redis server
// must be given as an array of bulk strings. If the argument isn't already in a
// slice or map it will be wrapped so that it is written as an Array of size
// one.
//
// Note that if a Message type is found it will *not* be encoded to a BulkStr,
// but will simply be passed through as whatever type it already represents.
func AppendArbitraryAsFlattenedStrings(buf []byte, m interface{}) []byte {
	fl := flattenedLength(m)
	buf = append(buf, arrayPrefix...)
	buf = strconv.AppendInt(buf, int64(fl), 10)
	buf = append(buf, delim...)

	return appendArb(buf, m, true, true)
}

// WriteArbitraryAsFlattenedStrings is similar to WriteArbitrary except that it
// will encode all types except Array as a BulkStr, converting the argument into
// a string first as necessary. It will also flatten any embedded arrays into a
// single long array. This is useful because commands to a redis server must be
// given as an array of bulk strings. If the argument isn't already in a slice
// or map it will be wrapped so that it is written as an Array of size one.
//
// Note that if a Message type is found it will *not* be encoded to a BulkStr,
// but will simply be passed through as whatever type it already represents.
func WriteArbitraryAsFlattenedStrings(w io.Writer, m interface{}) error {
	buf := AppendArbitraryAsFlattenedStrings(make([]byte, 0, 1024), m)
	_, err := w.Write(buf)
	return err
}

func appendArb(buf []byte, m interface{}, forceString, flattened bool) []byte {
	switch mt := m.(type) {
	case []byte:
		return appendStr(buf, mt)
	case string:
		return appendStr(buf, []byte(mt))
	case bool:
		if mt {
			return appendStr(buf, []byte("1"))
		} else {
			return appendStr(buf, []byte("0"))
		}
	case nil:
		if forceString {
			return appendStr(buf, []byte{})
		} else {
			return appendNil(buf)
		}
	case int:
		return appendInt(buf, int64(mt), forceString)
	case int8:
		return appendInt(buf, int64(mt), forceString)
	case int16:
		return appendInt(buf, int64(mt), forceString)
	case int32:
		return appendInt(buf, int64(mt), forceString)
	case int64:
		return appendInt(buf, mt, forceString)
	case uint:
		return appendInt(buf, int64(mt), forceString)
	case uint8:
		return appendInt(buf, int64(mt), forceString)
	case uint16:
		return appendInt(buf, int64(mt), forceString)
	case uint32:
		return appendInt(buf, int64(mt), forceString)
	case uint64:
		return appendInt(buf, int64(mt), forceString)
	case float32:
		ft := strconv.FormatFloat(float64(mt), 'f', -1, 32)
		return appendStr(buf, []byte(ft))
	case float64:
		ft := strconv.FormatFloat(mt, 'f', -1, 64)
		return appendStr(buf, []byte(ft))
	case error:
		if forceString {
			return appendStr(buf, []byte(mt.Error()))
		} else {
			return appendErr(buf, mt)
		}

	// For the following cases, where we are writing an array, we only append the
	// array header (a new array) if flattened is false, otherwise we just append
	// each element inline and assume the array header has already been written

	// We duplicate the below code here a bit, since this is the common case and
	// it'd be better to not get the reflect package involved here
	case []interface{}:
		l := len(mt)

		if !flattened {
			buf = append(buf, arrayPrefix...)
			buf = strconv.AppendInt(buf, int64(l), 10)
			buf = append(buf, delim...)
		}

		for i := 0; i < l; i++ {
			buf = appendArb(buf, mt[i], forceString, flattened)
		}
		return buf

	case *Message:
		buf = append(buf, mt.raw...)
		return buf

	default:
		// Fallback to reflect-based.
		switch reflect.TypeOf(m).Kind() {
		case reflect.Slice:
			rm := reflect.ValueOf(mt)
			l := rm.Len()

			if !flattened {
				buf = append(buf, arrayPrefix...)
				buf = strconv.AppendInt(buf, int64(l), 10)
				buf = append(buf, delim...)
			}

			for i := 0; i < l; i++ {
				vv := rm.Index(i).Interface()
				buf = appendArb(buf, vv, forceString, flattened)
			}
			return buf

		case reflect.Map:
			rm := reflect.ValueOf(mt)
			l := rm.Len() * 2

			if !flattened {
				buf = append(buf, arrayPrefix...)
				buf = strconv.AppendInt(buf, int64(l), 10)
				buf = append(buf, delim...)
			}

			keys := rm.MapKeys()
			for _, k := range keys {
				kv := k.Interface()
				vv := rm.MapIndex(k).Interface()
				buf = appendArb(buf, kv, forceString, flattened)
				buf = appendArb(buf, vv, forceString, flattened)
			}
			return buf

		default:
			return appendStr(buf, []byte(fmt.Sprint(m)))
		}
	}
}

var typeOfBytes = reflect.TypeOf([]byte(nil))

func flattenedLength(m interface{}) int {
	t := reflect.TypeOf(m)

	// If it's a byte-slice we don't want to flatten
	if t == typeOfBytes {
		return 1
	}

	total := 0

	switch t.Kind() {
	case reflect.Slice:
		rm := reflect.ValueOf(m)
		l := rm.Len()
		for i := 0; i < l; i++ {
			total += flattenedLength(rm.Index(i).Interface())
		}

	case reflect.Map:
		rm := reflect.ValueOf(m)
		keys := rm.MapKeys()
		for _, k := range keys {
			kv := k.Interface()
			vv := rm.MapIndex(k).Interface()
			total += flattenedLength(kv)
			total += flattenedLength(vv)
		}

	default:
		total++
	}

	return total
}

func appendStr(buf []byte, b []byte) []byte {
	buf = append(buf, bulkStrPrefix...)
	buf = strconv.AppendInt(buf, int64(len(b)), 10)
	buf = append(buf, delim...)
	buf = append(buf, b...)
	buf = append(buf, delim...)
	return buf
}

func appendErr(buf []byte, ierr error) []byte {
	buf = append(buf, errPrefix...)
	buf = append(buf, []byte(ierr.Error())...)
	buf = append(buf, delim...)
	return buf
}

func appendInt(buf []byte, i int64, forceString bool) []byte {
	if !forceString {
		buf = append(buf, intPrefix...)
	} else {
		// Really want to avoid alloating a new []byte. So I write the int to
		// the buf for the sole purpose of getting its length as a string, and
		// even though it'll be immediately overwritten right after and
		// AppendInt will be called again. This isn't great.
		tmpBuf := strconv.AppendInt(buf, i, 10)

		buf = append(buf, bulkStrPrefix...)
		buf = strconv.AppendInt(buf, int64(len(tmpBuf)-len(buf)+1), 10)
		buf = append(buf, delim...)
	}

	buf = strconv.AppendInt(buf, i, 10)
	buf = append(buf, delim...)
	return buf
}

var nilFormatted = []byte("$-1\r\n")

func appendNil(buf []byte) []byte {
	return append(buf, nilFormatted...)
}
