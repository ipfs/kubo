// The identify package handles how peers identify with eachother upon
// connection to the network
package identify

import (
	"bytes"
	"errors"

	proto "code.google.com/p/goprotobuf/proto"
	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// ErrUnsupportedKeyType is returned when a private key cast/type switch fails.
var ErrUnsupportedKeyType = errors.New("unsupported key type")

// Perform initial communication with this peer to share node ID's and
// initiate communication
func Handshake(self, remote *peer.Peer, in, out chan []byte) error {
	encoded, err := buildHandshake(self)
	if err != nil {
		return err
	}
	out <- encoded
	resp := <-in

	pbresp := new(Identify)
	err = proto.Unmarshal(resp, pbresp)
	if err != nil {
		return err
	}

	// Verify that the given ID matches their given public key
	if verifyErr := verifyID(peer.ID(pbresp.GetId()), pbresp.GetPubkey()); verifyErr != nil {
		return verifyErr
	}

	pubkey, err := ci.UnmarshalPublicKey(pbresp.GetPubkey())
	if err != nil {
		return err
	}

	// Challenge peer to ensure they own the given pubkey
	secret := self.PrivKey.GenSecret()
	encrypted, err := pubkey.Encrypt(secret)
	if err != nil {
		//... this is odd
		return err
	}

	out <- encrypted
	challenge := <-in

	// Decrypt challenge and send plaintext to partner
	plain, err := self.PrivKey.Decrypt(challenge)
	if err != nil {
		return err
	}

	out <- plain
	chalResp := <-in
	if !bytes.Equal(chalResp, secret) {
		return errors.New("Recieved incorrect challenge response!")
	}

	remote.ID = peer.ID(pbresp.GetId())
	remote.PubKey = pubkey
	u.DOut("[%s] identify: Got node id: %s\n", self.ID.Pretty(), remote.ID.Pretty())

	return nil
}

func buildHandshake(self *peer.Peer) ([]byte, error) {
	pkb, err := self.PubKey.Bytes()
	if err != nil {
		return nil, err
	}

	pmes := new(Identify)
	pmes.Id = []byte(self.ID)
	pmes.Pubkey = pkb

	encoded, err := proto.Marshal(pmes)
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

func verifyID(id peer.ID, pubkey []byte) error {
	hash, err := u.Hash(pubkey)
	if err != nil {
		return err
	}

	if id.Equal(peer.ID(hash)) {
		return nil
	}

	return errors.New("ID did not match public key!")
}

func IdFromPubKey(pk ci.PubKey) (peer.ID, error) {
	b, err := pk.Bytes()
	if err != nil {
		return nil, err
	}
	hash, err := u.Hash(b)
	if err != nil {
		return nil, err
	}
	return peer.ID(hash), nil
}
