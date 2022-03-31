package httpapi

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
)

var randomSha256MH = mh.Multihash{0x12, 0x20, 0x88, 0x82, 0x73, 0x37, 0x7c, 0xc1, 0xc9, 0x96, 0xad, 0xee, 0xd, 0x26, 0x84, 0x2, 0xc9, 0xc9, 0x5c, 0xf9, 0x5c, 0x4d, 0x9b, 0xc3, 0x3f, 0xfb, 0x4a, 0xd8, 0xaf, 0x28, 0x6b, 0xca, 0x1a, 0xf2}

func doParseIpldNotFoundTest(t *testing.T, original error) {
	originalMsg := original.Error()

	rebuilt := parseIPLDNotFoundWithFallbackToMSG(originalMsg)

	rebuiltMsg := rebuilt.Error()

	if originalMsg != rebuiltMsg {
		t.Errorf("expected message to be %q; got %q", originalMsg, rebuiltMsg)
	}

	originalNotFound := ipld.IsNotFound(original)
	rebuiltNotFound := ipld.IsNotFound(rebuilt)
	if originalNotFound != rebuiltNotFound {
		t.Errorf("for %q expected Ipld.IsNotFound to be %t; got %t", originalMsg, originalNotFound, rebuiltNotFound)
	}
}

func TestParseIPLDNotFound(t *testing.T) {
	if err := parseIPLDNotFoundWithFallbackToMSG(""); err != nil {
		t.Errorf("expected empty string to give no error; got %T %q", err, err.Error())
	}

	for _, wrap := range [...]string{
		"",
		"merkledag: %w",
		"testing: %w the test",
		"%w is wrong",
	} {
		for _, err := range [...]error{
			errors.New("file not found"),
			errors.New(" not found"),
			errors.New("Bad_CID not found"),
			errors.New("network connection timeout"),
			ipld.ErrNotFound{Cid: cid.Undef},
			ipld.ErrNotFound{Cid: cid.NewCidV0(randomSha256MH)},
			ipld.ErrNotFound{Cid: cid.NewCidV1(cid.Raw, randomSha256MH)},
		} {
			if wrap != "" {
				err = fmt.Errorf(wrap, err)
			}

			doParseIpldNotFoundTest(t, err)
		}
	}
}
