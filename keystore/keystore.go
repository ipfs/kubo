package keystore

import (
	"errors"
	"io/ioutil"
	"os"
	"path"

	ci "github.com/ipfs/go-ipfs/p2p/crypto"
)

type Keystore interface {
	GetKey(name string) (ci.PrivKey, error)
	PutKey(name string, k ci.PrivKey) error
}

type fsKeystore struct {
	baseDir string
}

func NewFsKeystore(dir string) Keystore {
	return &fsKeystore{dir}
}

func (ks *fsKeystore) GetKey(name string) (ci.PrivKey, error) {
	fi, err := os.Open(path.Join(ks.baseDir, name))
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	kb, err := ioutil.ReadAll(fi)
	if err != nil {
		return nil, err
	}

	return ci.UnmarshalPrivateKey(kb)
}

func (ks *fsKeystore) PutKey(name string, k ci.PrivKey) error {
	b, err := ci.MarshalPrivateKey(k)
	if err != nil {
		return err
	}

	fi, err := os.OpenFile(path.Join(ks.baseDir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	defer fi.Close()

	n, err := fi.Write(b)
	if err != nil {
		return err
	}

	if n != len(b) {
		return errors.New("key write failed to write enough bytes")
	}

	return nil
}
