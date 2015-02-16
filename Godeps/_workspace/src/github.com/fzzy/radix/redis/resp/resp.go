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

const (
	simpleStrPrefix = '+'
	errPrefix       = '-'
	intPrefix       = ':'
	bulkStrPrefix   = '$'
	arrayPrefix     = '*'
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
	b := append(make([]byte, 0, len(s) + 3), '+')
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
	case simpleStrPrefix:
		return readSimpleStr(r)
	case errPrefix:
		return readError(r)
	case intPrefix:
		return readInt(r)
	case bulkStrPrefix:
		return readBulkStr(r)
	case arrayPrefix:
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

// WriteMessage takes in the given Message and writes its encoded form to the
// given io.Writer
func WriteMessage(w io.Writer, m *Message) error {
	_, err := w.Write(m.raw)
	return err
}

// WriteArbitrary takes in any primitive golang value, or Message, and writes
// its encoded form to the given io.Writer, inferring types where appropriate.
func WriteArbitrary(w io.Writer, m interface{}) error {
	b := format(m, false)
	_, err := w.Write(b)
	return err
}

// WriteArbitraryAsString is similar to WriteArbitraryAsFlattenedString except
// that it won't flatten any embedded arrays.
func WriteArbitraryAsString(w io.Writer, m interface{}) error {
	b := format(m, true)
	_, err := w.Write(b)
	return err
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
	fm := flatten(m)
	return WriteArbitraryAsString(w, fm)
}

func format(m interface{}, forceString bool) []byte {
	switch mt := m.(type) {
	case []byte:
		return formatStr(mt)
	case string:
		return formatStr([]byte(mt))
	case bool:
		if mt {
			return formatStr([]byte("1"))
		} else {
			return formatStr([]byte("0"))
		}
	case nil:
		if forceString {
			return formatStr([]byte{})
		} else {
			return formatNil()
		}
	case int:
		return formatInt(int64(mt), forceString)
	case int8:
		return formatInt(int64(mt), forceString)
	case int16:
		return formatInt(int64(mt), forceString)
	case int32:
		return formatInt(int64(mt), forceString)
	case int64:
		return formatInt(mt, forceString)
	case uint:
		return formatInt(int64(mt), forceString)
	case uint8:
		return formatInt(int64(mt), forceString)
	case uint16:
		return formatInt(int64(mt), forceString)
	case uint32:
		return formatInt(int64(mt), forceString)
	case uint64:
		return formatInt(int64(mt), forceString)
	case float32:
		ft := strconv.FormatFloat(float64(mt), 'f', -1, 32)
		return formatStr([]byte(ft))
	case float64:
		ft := strconv.FormatFloat(mt, 'f', -1, 64)
		return formatStr([]byte(ft))
	case error:
		if forceString {
			return formatStr([]byte(mt.Error()))
		} else {
			return formatErr(mt)
		}

	// We duplicate the below code here a bit, since this is the common case and
	// it'd be better to not get the reflect package involved here
	case []interface{}:
		l := len(mt)
		b := make([]byte, 0, l*1024)
		b = append(b, '*')
		b = append(b, []byte(strconv.Itoa(l))...)
		b = append(b, []byte("\r\n")...)
		for i := 0; i < l; i++ {
			b = append(b, format(mt[i], forceString)...)
		}
		return b

	case *Message:
		return mt.raw

	default:
		// Fallback to reflect-based.
		switch reflect.TypeOf(m).Kind() {
		case reflect.Slice:
			rm := reflect.ValueOf(mt)
			l := rm.Len()
			b := make([]byte, 0, l*1024)
			b = append(b, '*')
			b = append(b, []byte(strconv.Itoa(l))...)
			b = append(b, []byte("\r\n")...)
			for i := 0; i < l; i++ {
				vv := rm.Index(i).Interface()
				b = append(b, format(vv, forceString)...)
			}

			return b
		case reflect.Map:
			rm := reflect.ValueOf(mt)
			l := rm.Len() * 2
			b := make([]byte, 0, l*1024)
			b = append(b, '*')
			b = append(b, []byte(strconv.Itoa(l))...)
			b = append(b, []byte("\r\n")...)
			keys := rm.MapKeys()
			for _, k := range keys {
				kv := k.Interface()
				vv := rm.MapIndex(k).Interface()
				b = append(b, format(kv, forceString)...)
				b = append(b, format(vv, forceString)...)
			}
			return b
		default:
			return formatStr([]byte(fmt.Sprint(m)))
		}
	}
}

var typeOfBytes = reflect.TypeOf([]byte(nil))

func flatten(m interface{}) []interface{} {
	t := reflect.TypeOf(m)

	// If it's a byte-slice we don't want to flatten
	if t == typeOfBytes {
		return []interface{}{m}
	}

	switch t.Kind() {
	case reflect.Slice:
		rm := reflect.ValueOf(m)
		l := rm.Len()
		ret := make([]interface{}, 0, l)
		for i := 0; i < l; i++ {
			ret = append(ret, flatten(rm.Index(i).Interface())...)
		}
		return ret

	case reflect.Map:
		rm := reflect.ValueOf(m)
		l := rm.Len() * 2
		keys := rm.MapKeys()
		ret := make([]interface{}, 0, l)
		for _, k := range keys {
			kv := k.Interface()
			vv := rm.MapIndex(k).Interface()
			ret = append(ret, flatten(kv)...)
			ret = append(ret, flatten(vv)...)
		}
		return ret

	default:
		return []interface{}{m}
	}
}

func formatStr(b []byte) []byte {
	l := strconv.Itoa(len(b))
	bs := make([]byte, 0, len(l)+len(b)+5)
	bs = append(bs, bulkStrPrefix)
	bs = append(bs, []byte(l)...)
	bs = append(bs, delim...)
	bs = append(bs, b...)
	bs = append(bs, delim...)
	return bs
}

func formatErr(ierr error) []byte {
	ierrstr := []byte(ierr.Error())
	bs := make([]byte, 0, len(ierrstr)+3)
	bs = append(bs, errPrefix)
	bs = append(bs, ierrstr...)
	bs = append(bs, delim...)
	return bs
}

func formatInt(i int64, forceString bool) []byte {
	istr := strconv.FormatInt(i, 10)
	if forceString {
		return formatStr([]byte(istr))
	}
	bs := make([]byte, 0, len(istr)+3)
	bs = append(bs, intPrefix)
	bs = append(bs, istr...)
	bs = append(bs, delim...)
	return bs
}

var nilFormatted = []byte("$-1\r\n")

func formatNil() []byte {
	return nilFormatted
}
