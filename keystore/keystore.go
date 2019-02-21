package keystore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	logging "github.com/ipfs/go-log"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	base32 "github.com/whyrusleeping/base32"
)

var log = logging.Logger("keystore")

// Keystore provides a key management interface
type Keystore interface {
	// Has returns whether or not a key exist in the Keystore
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

var ErrNoSuchKey = fmt.Errorf("no key by the given name was found")
var ErrKeyExists = fmt.Errorf("key by that name already exists, refusing to overwrite")

// FSKeystore is a keystore backed by files in a given directory stored on disk.
type FSKeystore struct {
	dir string
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("key names must be at least one character")
	}

	if strings.Contains(name, "/") {
		return fmt.Errorf("key names may not contain slashes")
	}

	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("key names may not begin with a period")
	}

	return nil
}

// NewKeystore is a factory for getting instance of Keystore interface implementation
func NewKeystore(dir string) (Keystore, error) {
	return NewEncodedFSKeystore(dir)
}

// NewEncodedFSKeystore is a factory for getting instance of EncodedFSKeystore
func NewEncodedFSKeystore(dir string) (*EncodedFSKeystore, error) {
	keystore, err := NewFSKeystore(dir)

	if err != nil {
		return nil, err
	}

	return &EncodedFSKeystore{keystore}, nil
}

func NewFSKeystore(dir string) (*FSKeystore, error) {
	_, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.Mkdir(dir, 0700); err != nil {
			return nil, err
		}
	}

	return &FSKeystore{dir}, nil
}

// Has returns whether or not a key exist in the Keystore
func (ks *FSKeystore) Has(name string) (bool, error) {
	kp := filepath.Join(ks.dir, name)

	_, err := os.Stat(kp)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if err := validateName(name); err != nil {
		return false, err
	}

	return true, nil
}

// Put stores a key in the Keystore, if a key with the same name already exists, returns ErrKeyExists
func (ks *FSKeystore) Put(name string, k ci.PrivKey) error {
	if err := validateName(name); err != nil {
		return err
	}

	b, err := k.Bytes()
	if err != nil {
		return err
	}

	kp := filepath.Join(ks.dir, name)

	_, err = os.Stat(kp)
	if err == nil {
		return ErrKeyExists
	} else if !os.IsNotExist(err) {
		return err
	}

	fi, err := os.Create(kp)
	if err != nil {
		return err
	}
	defer fi.Close()

	_, err = fi.Write(b)

	return err
}

// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
// otherwise.
func (ks *FSKeystore) Get(name string) (ci.PrivKey, error) {
	if err := validateName(name); err != nil {
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
	if err := validateName(name); err != nil {
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
		err := validateName(name)
		if err == nil {
			list = append(list, name)
		} else {
			log.Warnf("Ignoring the invalid keyfile: %s", name)
		}
	}

	return list, nil
}

const keyFilenamePrefix = "key_"

func encode(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("key name must be at least one character")
	}

	encodedName := base32.RawStdEncoding.EncodeToString([]byte(name))
	log.Debugf("Encoded key name: %s to: %s", name, encodedName)

	return keyFilenamePrefix + strings.ToLower(encodedName), nil
}

func decode(name string) (string, error) {
	if !strings.HasPrefix(name, keyFilenamePrefix) {
		return "", fmt.Errorf("key's filename has unexpected format")
	}

	nameWithoutPrefix := strings.ToUpper(name[len(keyFilenamePrefix):])
	data, err := base32.RawStdEncoding.DecodeString(nameWithoutPrefix)

	if err != nil {
		return "", err
	}

	decodedName := string(data[:])

	log.Debugf("Decoded key name: %s to: %s", name, decodedName)

	return decodedName, nil
}

// EncodedFSKeystore is extension of FSKeystore that encodes the key filenames in base32
type EncodedFSKeystore struct {
	*FSKeystore
}

// Has indicates if key is in keystore
func (ks *EncodedFSKeystore) Has(name string) (bool, error) {
	encodedName, err := encode(name)

	if err != nil {
		return false, err
	}

	return ks.FSKeystore.Has(encodedName)
}

// Put places key into the keystore
func (ks *EncodedFSKeystore) Put(name string, k ci.PrivKey) error {
	encodedName, err := encode(name)

	if err != nil {
		return err
	}

	return ks.FSKeystore.Put(encodedName, k)
}

// Get retrieves key by its name from the keystore
func (ks *EncodedFSKeystore) Get(name string) (ci.PrivKey, error) {
	encodedName, err := encode(name)

	if err != nil {
		return nil, err
	}

	return ks.FSKeystore.Get(encodedName)
}

// Delete removes key from the keystore
func (ks *EncodedFSKeystore) Delete(name string) error {
	encodedName, err := encode(name)

	if err != nil {
		return err
	}

	return ks.FSKeystore.Delete(encodedName)
}

// List returns list of all keys in keystore
func (ks *EncodedFSKeystore) List() ([]string, error) {
	dirs, err := ks.FSKeystore.List()

	if err != nil {
		return nil, err
	}

	list := make([]string, 0, len(dirs))

	for _, name := range dirs {
		decodedName, err := decode(name)
		if err == nil {
			list = append(list, decodedName)
		} else {
			log.Warningf("Ignoring keyfile with invalid encoded filename: %s", name)
		}
	}

	return list, nil
}
