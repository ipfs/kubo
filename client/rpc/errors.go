package rpc

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	mbase "github.com/multiformats/go-multibase"
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

// Assume CIDs break on:
// - Whitespaces: " \t\n\r\v\f"
// - Semicolon: ";" this is to parse ipld.ErrNotFound wrapped in multierr
// - Double Quotes: "\"" this is for parsing %q and %#v formatting.
const cidBreakSet = " \t\n\r\v\f;\""

func parseIPLDErrNotFound(msg string) (error, bool) {
	// The pattern we search for is:
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
		postIndex = strings.IndexFunc(msgPostKey, func(r rune) bool {
			return strings.ContainsAny(string(r), cidBreakSet)
		})
		if postIndex < 0 {
			// no breakage meaning the string look like this something + "ipld: could not find bafy"
			postIndex = len(msgPostKey)
		}

		cidStr := msgPostKey[:postIndex]

		var err error
		c, err = cid.Decode(cidStr)
		if err != nil {
			// failed to decode CID give up
			return nil, false
		}

		// check that the CID is either a CIDv0 or a base32 multibase
		// because that what ipld.ErrNotFound.Error() -> cid.Cid.String() do currently
		if c.Version() != 0 {
			baseRune, _ := utf8.DecodeRuneInString(cidStr)
			if baseRune == utf8.RuneError || baseRune != mbase.Base32 {
				// not a multibase we expect, give up
				return nil, false
			}
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
// That is needed to keep compatibility with code that use string.Contains(err.Error(), "blockstore: block not found")
// and code using ipld.ErrNotFound.
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

func parseBlockstoreNotFound(msg string) (error, bool) {
	if !strings.Contains(msg, "blockstore: block not found") {
		return nil, false
	}

	return blockstoreNotFoundMatchingIPLDErrNotFound{msg: msg}, true
}
