package peer

import (
	config "github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"
	ma "github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-multihash"

	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"math/big"
)

// ID is a byte slice representing the identity of a peer.
type ID mh.Multihash

// Utililty function for comparing two peer ID's
func (id *ID) Equal(other *ID) bool {
	return bytes.Equal(*id, *other)
}

// Map maps Key (string) : *Peer (slices are not comparable).
type Map map[u.Key]*Peer

// Peer represents the identity information of an IPFS Node, including
// ID, and relevant Addresses.
type Peer struct {
	ID        ID
	PubKey    []byte
	Addresses []*ma.Multiaddr

	curve   elliptic.Curve
	hellman []byte
	signing *ecdsa.PrivateKey
}

// Key returns the ID as a Key (string) for maps.
func (p *Peer) Key() u.Key {
	return u.Key(p.ID)
}

// AddAddress adds the given Multiaddr address to Peer's addresses.
func (p *Peer) AddAddress(a *ma.Multiaddr) {
	p.Addresses = append(p.Addresses, a)
}

// NetAddress returns the first Multiaddr found for a given network.
func (p *Peer) NetAddress(n string) *ma.Multiaddr {
	for _, a := range p.Addresses {
		ps, err := a.Protocols()
		if err != nil {
			continue // invalid addr
		}

		for _, p := range ps {
			if p.Name == n {
				return a
			}
		}
	}
	return nil
}

// Generates a shared secret key between us and the given public key.
func (p *Peer) Secret(pubKey []byte) ([]byte, error) {
	// Verify and unpack node's public key.
	curveSize := p.curve.Params().BitSize

	if len(pubKey) != (curveSize / 2) {
		return nil, errors.New("Malformed public key.")
	}

	bound := (curveSize / 8)
	x := big.NewInt(0)
	y := big.NewInt(0)

	x.SetBytes(pubKey[0:bound])
	y.SetBytes(pubKey[bound : bound*2])

	if !p.curve.IsOnCurve(x, y) {
		return nil, errors.New("Invalid public key.")
	}

	// Generate shared secret.
	secret, _ := p.curve.ScalarMult(x, y, p.hellman)

	return secret.Bytes(), nil
}

// Signs a given piece of data.
func (p *Peer) Sign(data []byte) ([]byte, error) {
	var out bytes.Buffer

	hash := sha256.New()
	hash.Write(data)

	r, s, err := ecdsa.Sign(rand.Reader, p.signing, hash.Sum(nil))
	if err != nil {
		return nil, err
	}

	out.Write(r.Bytes())
	out.Write(s.Bytes())

	return out.Bytes(), nil
}

// Verifies a signature on a given piece of data.
func (p *Peer) Verify(pubKey, data, sig []byte) (bool, error) {
	curveSize := p.curve.Params().BitSize

	// Verify and unpack public key.
	if len(pubKey) != (curveSize / 2) {
		return false, errors.New("Malformed public key.")
	}

	bound1 := (curveSize / 8)
	x := big.NewInt(0)
	y := big.NewInt(0)

	x.SetBytes(pubKey[bound1*2 : bound1*3])
	y.SetBytes(pubKey[bound1*3:])

	parsedPubKey := &ecdsa.PublicKey{p.curve, x, y}

	// Verify and unpack signature.
	if len(sig) != (curveSize / 4) {
		return false, errors.New("Malformed signature.")
	}

	bound2 := (curveSize / 8)
	r := big.NewInt(0)
	s := big.NewInt(0)

	r.SetBytes(sig[0:bound2])
	s.SetBytes(sig[bound2:])

	// Verify signature.
	hash := sha256.New()
	hash.Write(data)

	ok := ecdsa.Verify(parsedPubKey, hash.Sum(nil), r, s)

	return ok, nil
}

/**
* Generates a new, random peer identity.
*
* @return {Peer}
 */
func NewIdentity(cfg *config.Identity) (*Peer, error) {
	// Select curve given in config.
	var curve elliptic.Curve
	var pubKey bytes.Buffer

	switch cfg.Curve {
	case 224:
		curve = elliptic.P224()
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default:
		return nil, errors.New("Bad curve name in config.")
	}

	// Generate a random ECDH keypair.
	priv, x, y, err := elliptic.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}

	pubKey.Write(x.Bytes())
	pubKey.Write(y.Bytes())

	// Generate a random ECDSA keypair.
	signingPriv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}

	pubKey.Write(signingPriv.PublicKey.X.Bytes())
	pubKey.Write(signingPriv.PublicKey.Y.Bytes())

	// Generate peer ID
	hash := sha1.New() // THIS NEEDS TO BE SHA256
	hash.Write(pubKey.Bytes())

	id, err := mh.EncodeName(hash.Sum(nil), "sha1")
	if err != nil {
		return nil, err
	}

	return &Peer{
		id,
		pubKey.Bytes(),
		[]*ma.Multiaddr{},
		curve,
		priv,
		signingPriv,
	}, nil
}
