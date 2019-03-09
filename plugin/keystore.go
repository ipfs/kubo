package plugin

import (
	keystore "github.com/ipfs/go-ipfs/keystore"
	prompt "github.com/ipfs/go-prompt"
)

// PluginKeystore is an interface that can be implemented to new keystore
// backends.
type PluginKeystore interface {
	Plugin

	// KeystoreTypeName returns the the keystore's type. In addition to
	// loading the keystore plugin, the user must configure their go-ipfs
	// node to use the specified keystore backend.
	KeystoreTypeName() string

	// Open opens the keystore. Prompter may be nil if non-interactive.
	Open(
		repoPath string,
		config map[string]interface{},
		prompter prompt.Prompter,
	) (keystore.Keystore, error)
}
