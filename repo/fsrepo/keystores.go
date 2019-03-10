package fsrepo

import (
	"fmt"
	"path/filepath"

	keystore "github.com/ipfs/go-ipfs/keystore"

	prompt "github.com/ipfs/go-prompt"
)

type KeystoreConstructor func(
	repoPath string,
	cfg map[string]interface{},
	prompter prompt.Prompter,
) (keystore.Keystore, error)

var keystores = map[string]KeystoreConstructor{
	"memory": MemKeystoreFromConfig,
	"files":  FSKeystoreFromConfig,
}

func AddKeystore(name string, ctor KeystoreConstructor) error {
	_, ok := keystores[name]
	if ok {
		return fmt.Errorf("keystore %q registered more than once", name)
	}
	keystores[name] = ctor
	return nil
}

func FSKeystoreFromConfig(
	repo string,
	cfg map[string]interface{},
	_ prompt.Prompter,
) (keystore.Keystore, error) {
	path, ok := cfg["path"].(string)
	if !ok {
		return nil, fmt.Errorf("'path' field is missing or not a string")
	}
	return keystore.NewFSKeystore(filepath.Join(repo, path))
}

// MemKeystoreFromConfig opens an in-memory keystore based on the current
// config.
func MemKeystoreFromConfig(
	_ string,
	_ map[string]interface{},
	_ prompt.Prompter,
) (keystore.Keystore, error) {
	return keystore.NewMemKeystore(), nil
}
