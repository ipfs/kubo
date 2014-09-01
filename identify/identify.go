// The identify package handles how peers identify with eachother upon
// connection to the network
package identify

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"io/ioutil"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// Perform initial communication with this peer to share node ID's and
// initiate communication
func Handshake(self, remote *peer.Peer, in, out chan []byte) error {
	// TODO: make this more... secure.
	out <- self.ID
	resp := <-in
	remote.ID = peer.ID(resp)
	u.DOut("[%s] identify: Got node id: %s\n", self.ID.Pretty(), remote.ID.Pretty())

	return nil
}

type KeyPair struct {
	Pub  crypto.PublicKey
	Priv crypto.PrivateKey
}

func GenKeypair() (*KeyPair, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		Priv: priv,
		Pub:  priv.PublicKey,
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
