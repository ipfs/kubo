package fsrepo

import (
	"os"

	homedir "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/mitchellh/go-homedir"

	config "gx/ipfs/QmQSG7YCizeUH2bWatzp6uK9Vm3m7LA5jpxGa9QqgpNKw4/go-ipfs-config"
)

// BestKnownPath returns the best known fsrepo path. If the ENV override is
// present, this function returns that value. Otherwise, it returns the default
// repo path.
func BestKnownPath() (string, error) {
	ipfsPath := config.DefaultPathRoot
	if os.Getenv(config.EnvDir) != "" {
		ipfsPath = os.Getenv(config.EnvDir)
	}
	ipfsPath, err := homedir.Expand(ipfsPath)
	if err != nil {
		return "", err
	}
	return ipfsPath, nil
}
