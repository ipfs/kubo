// The identify package handles how peers identify with eachother upon
// connection to the network
package identify

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"io/ioutil"

	proto "code.google.com/p/goprotobuf/proto"
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

	pubkey, err := x509.ParsePKIXPublicKey(pbresp.GetPubkey())
	if err != nil {
		return err
	}

	// Challenge peer to ensure they own the given pubkey
	secret := make([]byte, 16)
	rand.Read(secret)
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubkey.(*rsa.PublicKey), secret)
	if err != nil {
		//... this is odd
		return err
	}

	out <- encrypted
	challenge := <-in

	// Decrypt challenge and send plaintext to partner
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, self.PrivKey.(*rsa.PrivateKey), challenge)
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
	pkb, err := x509.MarshalPKIXPublicKey(self.PubKey)
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

type KeyPair struct {
	Pub  crypto.PublicKey
	Priv crypto.PrivateKey
}

func GenKeypair(bits int) (*KeyPair, error) {
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		Priv: priv,
		Pub:  &priv.PublicKey,
	}, nil
}

func LoadKeypair(dir string) (*KeyPair, error) {
	var kp KeyPair
	pk_b, err := ioutil.ReadFile(dir + "/priv.key")
	if err != nil {
		return nil, err
	}

	priv, err := x509.ParsePKCS1PrivateKey(pk_b)
	if err != nil {
		return nil, err
	}

	kp.Priv = priv
	kp.Pub = priv.PublicKey

	return &kp, nil
}

func (pk *KeyPair) ID() (peer.ID, error) {
	pub_b, err := x509.MarshalPKIXPublicKey(pk.Pub)
	if err != nil {
		return nil, err
	}
	hash, err := u.Hash(pub_b)
	if err != nil {
		return nil, err
	}
	return peer.ID(hash), nil
}

func (pk *KeyPair) PrivBytes() []byte {
	switch k := pk.Priv.(type) {
	case *rsa.PrivateKey:
		return x509.MarshalPKCS1PrivateKey(k)
	default:
		panic("Unsupported private key type.")
	}
}

func (kp *KeyPair) Save(dir string) error {
	switch k := kp.Priv.(type) {
	case *rsa.PrivateKey:
		err := k.Validate()
		if err != nil {
			return err
		}
		pk_b := x509.MarshalPKCS1PrivateKey(k)
		err = ioutil.WriteFile(dir+"/priv.key", pk_b, 0600)
		return err
	default:
		return errors.New("invalid private key type.")
	}
}
