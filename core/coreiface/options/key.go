package options

import "fmt"

const (
	RSAKey     = "rsa"
	Ed25519Key = "ed25519"

	DefaultRSALen = 2048

	// ed25519 and secp256k1 private keys are always 256 bits; unlike RSA
	// they do not have a variable size.
	ed25519KeyBits   = 256
	secp256k1KeyBits = 256
)

type KeyGenerateSettings struct {
	Algorithm string
	Size      int
}

type KeyRenameSettings struct {
	Force bool
}

type (
	KeyGenerateOption func(*KeyGenerateSettings) error
	KeyRenameOption   func(*KeyRenameSettings) error
)

func KeyGenerateOptions(opts ...KeyGenerateOption) (*KeyGenerateSettings, error) {
	options := &KeyGenerateSettings{
		Algorithm: RSAKey,
		Size:      -1,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

func KeyRenameOptions(opts ...KeyRenameOption) (*KeyRenameSettings, error) {
	options := &KeyRenameSettings{
		Force: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type keyOpts struct{}

var Key keyOpts

// Type is an option for Key.Generate which specifies which algorithm
// should be used for the key. Default is options.RSAKey
//
// Supported key types:
// * options.RSAKey
// * options.Ed25519Key
func (keyOpts) Type(algorithm string) KeyGenerateOption {
	return func(settings *KeyGenerateSettings) error {
		settings.Algorithm = algorithm
		return nil
	}
}

// Size is an option for Key.Generate which specifies the size of the key to
// generate. Default is -1.
//
// A value of -1 means "use the default size for the key type":
//   - 2048 for RSA
//
// ed25519 and secp256k1 keys are always 256 bits; a size is accepted only
// when it matches.
func (keyOpts) Size(size int) KeyGenerateOption {
	return func(settings *KeyGenerateSettings) error {
		settings.Size = size
		return nil
	}
}

// CheckKeySize validates a requested key size for the given algorithm. RSA
// accepts any size (callers apply DefaultRSALen when it is unset). ed25519 and
// secp256k1 have a fixed size, so a size is accepted only when it is unset (-1)
// or equals that size, and rejected otherwise.
func CheckKeySize(algorithm string, size int) error {
	if size == -1 {
		return nil
	}
	var fixed int
	switch algorithm {
	case "ed25519":
		fixed = ed25519KeyBits
	case "secp256k1":
		fixed = secp256k1KeyBits
	default:
		return nil
	}
	if size != fixed {
		return fmt.Errorf("invalid key size %d: %s keys are always %d bits", size, algorithm, fixed)
	}
	return nil
}

// Force is an option for Key.Rename which specifies whether to allow to
// replace existing keys.
func (keyOpts) Force(force bool) KeyRenameOption {
	return func(settings *KeyRenameSettings) error {
		settings.Force = force
		return nil
	}
}
