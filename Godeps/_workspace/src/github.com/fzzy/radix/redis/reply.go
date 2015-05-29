package redis

import (
	"errors"
	"strconv"
	"strings"
)

// A CmdError implements the error interface and is what is returned when the
// server returns an error on the application level (e.g. key doesn't exist or
// is the wrong type), as opposed to a connection/transport error.
//
// You can test if a reply is a CmdError like so:
//
//	r := conn.Cmd("GET", "key-which-isnt-a-string")
//	if r.Err != nil {
//		if cerr, ok := r.Err.(*redis.CmdError); ok {
//			// Is CmdError
//		} else {
//			// Is other error
//		}
//	}
type CmdError struct {
	Err error
}

func (cerr *CmdError) Error() string {
	return cerr.Err.Error()
}

// Returns true if error returned was due to the redis server being read only
func (cerr *CmdError) Readonly() bool {
	return strings.HasPrefix(cerr.Err.Error(), "READONLY")
}

//* Reply

/*
ReplyType describes type of a reply.

Possible values are:

StatusReply --  status reply
ErrorReply -- error reply
IntegerReply -- integer reply
NilReply -- nil reply
BulkReply -- bulk reply
MultiReply -- multi bulk reply
*/
type ReplyType uint8

const (
	StatusReply ReplyType = iota
	ErrorReply
	IntegerReply
	NilReply
	BulkReply
	MultiReply
)

// Reply holds a Redis reply.
type Reply struct {
	Type  ReplyType // Reply type
	Elems []*Reply  // Sub-replies
	Err   error     // Reply error
	buf   []byte
	int   int64
}

// Bytes returns the reply value as a byte string or
// an error, if the reply type is not StatusReply or BulkReply.
func (r *Reply) Bytes() ([]byte, error) {
	if r.Type == ErrorReply {
		return nil, r.Err
	}
	if !(r.Type == StatusReply || r.Type == BulkReply) {
		return nil, errors.New("string value is not available for this reply type")
	}

	return r.buf, nil
}

// Str is a convenience method for calling Reply.Bytes() and converting it to string
func (r *Reply) Str() (string, error) {
	b, err := r.Bytes()
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Int64 returns the reply value as a int64 or an error,
// if the reply type is not IntegerReply or the reply type
// BulkReply could not be parsed to an int64.
func (r *Reply) Int64() (int64, error) {
	if r.Type == ErrorReply {
		return 0, r.Err
	}
	if r.Type != IntegerReply {
		s, err := r.Str()
		if err == nil {
			i64, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return 0, errors.New("failed to parse integer value from string value")
			} else {
				return i64, nil
			}
		}

		return 0, errors.New("integer value is not available for this reply type")
	}

	return r.int, nil
}

// Int is a convenience method for calling Reply.Int64() and converting it to int.
func (r *Reply) Int() (int, error) {
	i64, err := r.Int64()
	if err != nil {
		return 0, err
	}

	return int(i64), nil
}

// Float64 returns the reply value as a float64 or an error,
// if the reply type is not BulkReply or the reply type
// BulkReply could not be parsed to an float64.
func (r *Reply) Float64() (float64, error) {
	if r.Type == ErrorReply {
		return 0, r.Err
	}
	if r.Type == BulkReply {
		s, err := r.Str()
		if err != nil {
			return 0, err
		}
		f64, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, errors.New("failed to parse float value from string value")
		} else {
			return f64, nil
		}
	}

	return 0, errors.New("float value is not available for this reply type")
}

// Bool returns false, if the reply value equals to 0 or "0", otherwise true; or
// an error, if the reply type is not IntegerReply or BulkReply.
func (r *Reply) Bool() (bool, error) {
	if r.Type == ErrorReply {
		return false, r.Err
	}
	i, err := r.Int()
	if err == nil {
		if i == 0 {
			return false, nil
		}

		return true, nil
	}

	s, err := r.Str()
	if err == nil {
		if s == "0" {
			return false, nil
		}

		return true, nil
	}

	return false, errors.New("boolean value is not available for this reply type")
}

// List returns a multi bulk reply as a slice of strings or an error.
// The reply type must be MultiReply and its elements' types must all be either BulkReply or NilReply.
// Nil elements are returned as empty strings.
// Useful for list commands.
func (r *Reply) List() ([]string, error) {
	// Doing all this in two places instead of just calling ListBytes() so we don't have
	// to iterate twice
	if r.Type == ErrorReply {
		return nil, r.Err
	}
	if r.Type != MultiReply {
		return nil, errors.New("reply type is not MultiReply")
	}

	strings := make([]string, len(r.Elems))
	for i, v := range r.Elems {
		if v.Type == BulkReply {
			strings[i] = string(v.buf)
		} else if v.Type == NilReply {
			strings[i] = ""
		} else {
			return nil, errors.New("element type is not BulkReply or NilReply")
		}
	}

	return strings, nil
}

// ListBytes returns a multi bulk reply as a slice of bytes slices or an error.
// The reply type must be MultiReply and its elements' types must all be either BulkReply or NilReply.
// Nil elements are returned as nil.
// Useful for list commands.
func (r *Reply) ListBytes() ([][]byte, error) {
	if r.Type == ErrorReply {
		return nil, r.Err
	}
	if r.Type != MultiReply {
		return nil, errors.New("reply type is not MultiReply")
	}

	bufs := make([][]byte, len(r.Elems))
	for i, v := range r.Elems {
		if v.Type == BulkReply {
			bufs[i] = v.buf
		} else if v.Type == NilReply {
			bufs[i] = nil
		} else {
			return nil, errors.New("element type is not BulkReply or NilReply")
		}
	}

	return bufs, nil
}

// Hash returns a multi bulk reply as a map[string]string or an error.
// The reply type must be MultiReply,
// it must have an even number of elements,
// they must be in a "key value key value..." order and
// values must all be either BulkReply or NilReply.
// Nil values are returned as empty strings.
// Useful for hash commands.
func (r *Reply) Hash() (map[string]string, error) {
	if r.Type == ErrorReply {
		return nil, r.Err
	}
	rmap := map[string]string{}

	if r.Type != MultiReply {
		return nil, errors.New("reply type is not MultiReply")
	}

	if len(r.Elems)%2 != 0 {
		return nil, errors.New("reply has odd number of elements")
	}

	for i := 0; i < len(r.Elems)/2; i++ {
		var val string

		key, err := r.Elems[i*2].Str()
		if err != nil {
			return nil, errors.New("key element has no string reply")
		}

		v := r.Elems[i*2+1]
		if v.Type == BulkReply {
			val = string(v.buf)
			rmap[key] = val
		} else if v.Type == NilReply {
		} else {
			return nil, errors.New("value element type is not BulkReply or NilReply")
		}
	}

	return rmap, nil
}

// String returns a string representation of the reply and its sub-replies.
// This method is for debugging.
// Use method Reply.Str() for reading string reply.
func (r *Reply) String() string {
	switch r.Type {
	case ErrorReply:
		return r.Err.Error()
	case StatusReply:
		fallthrough
	case BulkReply:
		return string(r.buf)
	case IntegerReply:
		return strconv.FormatInt(r.int, 10)
	case NilReply:
		return "<nil>"
	case MultiReply:
		s := "[ "
		for _, e := range r.Elems {
			s = s + e.String() + " "
		}
		return s + "]"
	}

	// This should never execute
	return ""
}
