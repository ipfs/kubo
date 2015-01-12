package fsrepo

import (
	"io"

	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
)

type FSRepo struct {
	path   string
	config config.Config
}

func At(path string) *FSRepo {
	return &FSRepo{
		path: path,
	}
}

func (r *FSRepo) Open() error {
	// TODO may need to check that directory is writeable
	// TODO acquire repo lock
	return nil
}

func (r *FSRepo) SetConfig(conf *config.Config) error {
	configFilename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	if err := config.WriteConfigFile(configFilename, conf); err != nil {
		return err
	}
	r.config = *conf // copy so caller cannot modify the private config
	return nil
}

func (r *FSRepo) Close() error {
	return nil // TODO release repo lock
}

var _ io.Closer = &FSRepo{}

// ConfigIsInitialized returns true if the config exists in provided |path|.
func ConfigIsInitialized(path string) bool {
	configFilename, err := config.Filename(path)
	if err != nil {
		return false
	}
	if !util.FileExists(configFilename) {
		return false
	}
	return true
}
