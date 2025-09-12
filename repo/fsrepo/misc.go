package fsrepo

import (
	"os"

	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/misc/fsutil"
)

// BestKnownPath returns the best known fsrepo path. If the ENV override is
// present, this function returns that value. Otherwise, it returns the default
// repo path.
func BestKnownPath() (string, error) {
	ipfsPath := config.DefaultPathRoot
	if os.Getenv(config.EnvDir) != "" {
		ipfsPath = os.Getenv(config.EnvDir)
	}
	ipfsPath, err := fsutil.ExpandHome(ipfsPath)
	if err != nil {
		return "", err
	}
	return ipfsPath, nil
}
