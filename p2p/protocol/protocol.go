package protocol

import (
	"io"

	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
)

// ID is an identifier used to write protocol headers in streams.
type ID string

// These are reserved protocol.IDs.
const (
	TestingID ID = "/p2p/_testing"
)

// WriteHeader writes a protocol.ID header to an io.Writer. This is so
// multiple protocols can be multiplexed on top of the same transport.
//
// We use go-msgio varint encoding:
//   <varint length><string name>\n
// (the varint includes the \n)
func WriteHeader(w io.Writer, id ID) error {
	vw := msgio.NewVarintWriter(w)
	s := string(id) + "\n" // add \n
	return vw.WriteMsg([]byte(s))
}

// ReadHeader reads a protocol.ID header from an io.Reader. This is so
// multiple protocols can be multiplexed on top of the same transport.
// See WriteHeader.
func ReadHeader(r io.Reader) (ID, error) {
	vr := msgio.NewVarintReader(r)
	msg, err := vr.ReadMsg()
	if err != nil {
		return ID(""), err
	}
	msg = msg[:len(msg)-1] // remove \n
	return ID(msg), nil
}
