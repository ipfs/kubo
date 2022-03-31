package httpapi

import (
	"errors"
	"strings"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

// This file handle parsing and returning the correct ABI based errors from error messages

type prePostWrappedNotFoundError struct {
	pre  string
	post string

	wrapped ipld.ErrNotFound
}

func (e prePostWrappedNotFoundError) String() string {
	return e.Error()
}

func (e prePostWrappedNotFoundError) Error() string {
	return e.pre + e.wrapped.Error() + e.post
}

func (e prePostWrappedNotFoundError) Unwrap() error {
	return e.wrapped
}

func parseErrNotFoundWithFallbackToMSG(msg string) error {
	err, handled := parseErrNotFound(msg)
	if handled {
		return err
	}

	return errors.New(msg)
}

func parseErrNotFoundWithFallbackToError(msg error) error {
	err, handled := parseErrNotFound(msg.Error())
	if handled {
		return err
	}

	return msg
}

// Use a string to move it into RODATA
// print("".join("\\x01" if chr(i) not in string.ascii_letters + string.digits else "\\x00" for i in range(ord('z')+1)))
const notAsciiLetterOrDigitsLUT = "\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01\x01\x01\x01\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01\x01\x01\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"

func notAsciiLetterOrDigits(r rune) bool {
	if r > 'z' {
		return true
	}

	return notAsciiLetterOrDigitsLUT[r] > 0
}

//lint:ignore ST1008 this function is not using the error as a mean to return failure but it massages it to return the correct type
func parseErrNotFound(msg string) (error, bool) {
	if msg == "" {
		return nil, true // Fast path
	}

	if err, handled := parseIPLDErrNotFound(msg); handled {
		return err, true
	}

	if err, handled := parseBlockstoreNotFound(msg); handled {
		return err, true
	}

	return nil, false
}

//lint:ignore ST1008 using error as values
func parseIPLDErrNotFound(msg string) (error, bool) {
	// The patern we search for is:
	const ipldErrNotFoundKey = "ipld: could not find " /*CID*/
	// We try to parse the CID, if it's invalid we give up and return a simple text error.
	// We also accept "node" in place of the CID because that means it's an Undefined CID.

	keyIndex := strings.Index(msg, ipldErrNotFoundKey)

	if keyIndex < 0 { // Unknown error
		return nil, false
	}

	cidStart := keyIndex + len(ipldErrNotFoundKey)

	msgPostKey := msg[cidStart:]
	var c cid.Cid
	var postIndex int
	if strings.HasPrefix(msgPostKey, "node") {
		// Fallback case
		c = cid.Undef
		postIndex = len("node")
	} else {
		// Assume that CIDs only contain a-zA-Z0-9 characters.
		// This is true because go-ipld-format use go-cid#Cid.String which use base{3{2,6},58}.
		postIndex = strings.IndexFunc(msgPostKey, notAsciiLetterOrDigits)
		if postIndex < 0 {
			postIndex = len(msgPostKey)
		}

		var err error
		c, err = cid.Decode(msgPostKey[:postIndex])
		if err != nil {
			// Unknown
			return nil, false
		}
	}

	err := ipld.ErrNotFound{Cid: c}
	pre := msg[:keyIndex]
	post := msgPostKey[postIndex:]

	if len(pre) > 0 || len(post) > 0 {
		return prePostWrappedNotFoundError{
			pre:     pre,
			post:    post,
			wrapped: err,
		}, true
	}

	return err, true
}

// This is a simple error type that just return msg as Error().
// But that also match ipld.ErrNotFound when called with Is(err).
// That is needed to keep compatiblity with code that use string.Contains(err.Error(), "blockstore: block not found")
// and code using ipld.ErrNotFound
type blockstoreNotFoundMatchingIPLDErrNotFound struct {
	msg string
}

func (e blockstoreNotFoundMatchingIPLDErrNotFound) String() string {
	return e.Error()
}

func (e blockstoreNotFoundMatchingIPLDErrNotFound) Error() string {
	return e.msg
}

func (e blockstoreNotFoundMatchingIPLDErrNotFound) Is(err error) bool {
	_, ok := err.(ipld.ErrNotFound)
	return ok
}

//lint:ignore ST1008 using error as values
func parseBlockstoreNotFound(msg string) (error, bool) {
	if !strings.Contains(msg, "blockstore: block not found") {
		return nil, false
	}

	return blockstoreNotFoundMatchingIPLDErrNotFound{msg: msg}, true
}
