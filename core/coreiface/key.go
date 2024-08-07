package iface

import (
	"context"

	"github.com/ipfs/boxo/path"

	"github.com/ipfs/kubo/core/coreiface/options"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Key specifies the interface to Keys in KeyAPI Keystore
type Key interface {
	// Key returns key name
	Name() string

	// Path returns key path
	Path() path.Path

	// ID returns key PeerID
	ID() peer.ID
}

// KeyAPI specifies the interface to Keystore
type KeyAPI interface {
	// Generate generates new key, stores it in the keystore under the specified
	// name and returns a base58 encoded multihash of it's public key
	Generate(ctx context.Context, name string, opts ...options.KeyGenerateOption) (Key, error)

	// Rename renames oldName key to newName. Returns the key and whether another
	// key was overwritten, or an error
	Rename(ctx context.Context, oldName string, newName string, opts ...options.KeyRenameOption) (Key, bool, error)

	// List lists keys stored in keystore
	List(ctx context.Context) ([]Key, error)

	// Self returns the 'main' node key
	Self(ctx context.Context) (Key, error)

	// Remove removes keys from keystore. Returns ipns path of the removed key
	Remove(ctx context.Context, name string) (Key, error)

	// Sign signs the given data with the key named name. Returns the key used
	// for signing, the signature, and an error.
	Sign(ctx context.Context, name string, data []byte) (Key, []byte, error)

	// Verify verifies if the given data and signatures match. Returns the key used
	// for verification, whether signature and data match, and an error.
	Verify(ctx context.Context, keyOrName string, signature, data []byte) (Key, bool, error)
}
