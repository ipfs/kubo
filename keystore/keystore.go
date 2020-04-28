package keystore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	base32 "encoding/base32"

	logging "github.com/ipfs/go-log"
	ci "github.com/libp2p/go-libp2p-core/crypto"
)

var log = logging.Logger("keystore")

var codec = base32.StdEncoding.WithPadding(base32.NoPadding)

// Keystore provides a key management interface
type Keystore interface {
	// Has returns whether or not a key exists in the Keystore
	Has(string) (bool, error)
	// Put stores a key in the Keystore, if a key with the same name already exists, returns ErrKeyExists
	Put(string, ci.PrivKey) error
	// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
	// otherwise.
	Get(string) (ci.PrivKey, error)
	// Delete removes a key from the Keystore
	Delete(string) error
	// List returns a list of key identifier
	List() ([]string, error)
}

// ErrNoSuchKey is an error message returned when no key of a given name was found.
var ErrNoSuchKey = fmt.Errorf("no key by the given name was found")

// ErrKeyExists is an error message returned when a key already exists
var ErrKeyExists = fmt.Errorf("key by that name already exists, refusing to overwrite")

const keyFilenamePrefix = "key_"

// FSKeystore is a keystore backed by files in a given directory stored on disk.
type FSKeystore struct {
	dir string
}

// NewFSKeystore returns a new filesystem-backed keystore.
func NewFSKeystore(dir string) (*FSKeystore, error) {
	err := os.Mkdir(dir, 0700)
	switch {
	case os.IsExist(err):
	case err == nil:
	default:
		return nil, err
	}
	return &FSKeystore{dir}, nil
}

// Has returns whether or not a key exists in the Keystore
func (ks *FSKeystore) Has(name string) (bool, error) {
	name, err := encode(name)
	if err != nil {
		return false, err
	}

	kp := filepath.Join(ks.dir, name)

	_, err = os.Stat(kp)

	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// Put stores a key in the Keystore, if a key with the same name already exists, returns ErrKeyExists
func (ks *FSKeystore) Put(name string, k ci.PrivKey) error {
	name, err := encode(name)
	if err != nil {
		return err
	}

	b, err := ci.MarshalPrivateKey(k)
	if err != nil {
		return err
	}

	kp := filepath.Join(ks.dir, name)

	fi, err := os.OpenFile(kp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0400)
	if err != nil {
		if os.IsExist(err) {
			err = ErrKeyExists
		}
		return err
	}
	defer fi.Close()

	_, err = fi.Write(b)

	return err
}

// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
// otherwise.
func (ks *FSKeystore) Get(name string) (ci.PrivKey, error) {
	name, err := encode(name)
	if err != nil {
		return nil, err
	}

	kp := filepath.Join(ks.dir, name)

	data, err := ioutil.ReadFile(kp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSuchKey
		}
		return nil, err
	}

	return ci.UnmarshalPrivateKey(data)
}

// Delete removes a key from the Keystore
func (ks *FSKeystore) Delete(name string) error {
	name, err := encode(name)
	if err != nil {
		return err
	}

	kp := filepath.Join(ks.dir, name)

	return os.Remove(kp)
}

// List return a list of key identifier
func (ks *FSKeystore) List() ([]string, error) {
	dir, err := os.Open(ks.dir)
	if err != nil {
		return nil, err
	}

	dirs, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	list := make([]string, 0, len(dirs))

	for _, name := range dirs {
		decodedName, err := decode(name)
		if err == nil {
			list = append(list, decodedName)
		} else {
			log.Errorf("Ignoring keyfile with invalid encoded filename: %s", name)
		}
	}

	return list, nil
}

func encode(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("key name must be at least one character")
	}

	encodedName := codec.EncodeToString([]byte(name))
	log.Debugf("Encoded key name: %s to: %s", name, encodedName)

	return keyFilenamePrefix + strings.ToLower(encodedName), nil
}

func decode(name string) (string, error) {
	if !strings.HasPrefix(name, keyFilenamePrefix) {
		return "", fmt.Errorf("key's filename has unexpected format")
	}

	nameWithoutPrefix := strings.ToUpper(name[len(keyFilenamePrefix):])
	decodedName, err := codec.DecodeString(nameWithoutPrefix)
	if err != nil {
		return "", err
	}

	log.Debugf("Decoded key name: %s to: %s", name, decodedName)

	return string(decodedName), nil
}
