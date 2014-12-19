// package peer implements an object used to represent peers in the ipfs network.
package peer

import (
	"fmt"

	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	ic "github.com/jbenet/go-ipfs/crypto"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("peer")

// ID represents the identity of a peer.
type ID string

// Pretty returns a b58-encoded string of the ID
func (id ID) Pretty() string {
	return IDB58Encode(id)
}

func (id ID) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"peerID": id.Pretty(),
	}
}

// String prints out the peer.
//
// TODO(brian): ensure correctness at ID generation and
// enforce this by only exposing functions that generate
// IDs safely. Then any peer.ID type found in the
// codebase is known to be correct.
func (id ID) String() string {
	pid := string(id)
	maxRunes := 6
	if len(pid) < maxRunes {
		maxRunes = len(pid)
	}
	return fmt.Sprintf("<peer.ID %s>", pid[:maxRunes])
}

// IDFromString cast a string to ID type, and validate
// the id to make sure it is a multihash.
func IDFromString(s string) (ID, error) {
	if _, err := mh.Cast([]byte(s)); err != nil {
		return ID(""), err
	}
	return ID(s), nil
}

// IDFromBytes cast a string to ID type, and validate
// the id to make sure it is a multihash.
func IDFromBytes(b []byte) (ID, error) {
	if _, err := mh.Cast(b); err != nil {
		return ID(""), err
	}
	return ID(b), nil
}

// IDB58Decode returns a b58-decoded Peer
func IDB58Decode(s string) (ID, error) {
	m, err := mh.FromB58String(s)
	if err != nil {
		return "", err
	}
	return ID(m), err
}

// IDB58Encode returns b58-encoded string
func IDB58Encode(id ID) string {
	return b58.Encode([]byte(id))
}

// IDFromPubKey returns the Peer ID corresponding to pk
func IDFromPubKey(pk ic.PubKey) (ID, error) {
	b, err := pk.Bytes()
	if err != nil {
		return "", err
	}
	hash := u.Hash(b)
	return ID(hash), nil
}

// Map maps a Peer ID to a struct.
type Set map[ID]struct{}
