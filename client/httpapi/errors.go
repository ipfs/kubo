package httpapi

import (
	"errors"
	"strings"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

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

func parseIPLDNotFoundWithFallbackToMSG(msg string) error {
	err, handled := parseIPLDNotFound(msg)
	if handled {
		return err
	}

	return errors.New(msg)
}

func parseIPLDNotFoundWithFallbackToError(msg error) error {
	err, handled := parseIPLDNotFound(msg.Error())
	if handled {
		return err
	}

	return msg
}

// This file handle parsing and returning the correct ABI based errors from error messages
//lint:ignore ST1008 this function is not using the error as a mean to return failure but it massages it to return the correct type
func parseIPLDNotFound(msg string) (error, bool) {
	if msg == "" {
		return nil, true // Fast path
	}

	// The patern we search for is:
	//   node not found (fallback)
	// or
	//   CID not found (here we parse the CID)
	notFoundIndex := strings.LastIndex(msg, " not found")

	if notFoundIndex == -1 {
		// Unknown, ot found not found
		return nil, false
	}

	preNotFound := msg[:notFoundIndex]

	var c cid.Cid
	var preIndex int
	if strings.HasSuffix(preNotFound, "node") {
		// Fallback case
		c = cid.Undef
		preIndex = notFoundIndex - len("node")
	} else {
		// Assume that CIDs does not include whitespace to pull out the CID
		preIndex = strings.LastIndexByte(preNotFound, ' ')
		// + 1 is to normalise not founds to zeros and point to the start of the CID, not the previous space
		preIndex++
		var err error
		c, err = cid.Decode(preNotFound[preIndex:])
		if err != nil {
			// Unknown
			return nil, false
		}
	}

	postIndex := notFoundIndex + len(" not found")

	err := ipld.ErrNotFound{Cid: c}

	pre := msg[:preIndex]
	post := msg[postIndex:]

	if len(pre) > 0 || len(post) > 0 {
		// We have some text to wrap arround the ErrNotFound one
		return prePostWrappedNotFoundError{
			pre:     pre,
			post:    post,
			wrapped: err,
		}, true
	}

	return err, true
}
