package iface

import (
	"context"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

// Key specifies the interface to Keys in KeyAPI Keystore
type Key interface {
	// Key returns key name
	Name() string
	// Path returns key path
	Path() Path
}

// KeyAPI specifies the interface to Keystore
type KeyAPI interface {
	// Generate generates new key, stores it in the keystore under the specified
	// name and returns a base58 encoded multihash of it's public key
	Generate(ctx context.Context, name string, opts ...options.KeyGenerateOption) (Key, error)

	// WithType is an option for Generate which specifies which algorithm
	// should be used for the key. Default is options.RSAKey
	//
	// Supported key types:
	// * options.RSAKey
	// * options.Ed25519Key
	WithType(algorithm string) options.KeyGenerateOption

	// WithSize is an option for Generate which specifies the size of the key to
	// generated. Default is -1
	//
	// value of -1 means 'use default size for key type':
	//  * 2048 for RSA
	WithSize(size int) options.KeyGenerateOption

	// Rename renames oldName key to newName. Returns the key and whether another
	// key was overwritten, or an error
	Rename(ctx context.Context, oldName string, newName string, opts ...options.KeyRenameOption) (Key, bool, error)

	// WithForce is an option for Rename which specifies whether to allow to
	// replace existing keys.
	WithForce(force bool) options.KeyRenameOption

	// List lists keys stored in keystore
	List(ctx context.Context) ([]Key, error)

	// Remove removes keys from keystore. Returns ipns path of the removed key
	Remove(ctx context.Context, name string) (Path, error)
}
