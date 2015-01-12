package fsrepo

import (
	"io"
	"os"
	"path/filepath"

	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/debugerror"
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
	// check repo path, then check all constituent parts.
	// TODO acquire repo lock
	// TODO if err := initCheckDir(logpath); err != nil { // }
	if err := initCheckDir(r.path); err != nil {
		return err
	}

	// datastore
	dspath, err := config.DataStorePath("")
	if err != nil {
		return err
	}
	if err := initCheckDir(dspath); err != nil {
		return debugerror.Errorf("datastore: %s", err)
	}

	logpath, err := config.LogsPath("")
	if err != nil {
		return debugerror.Wrap(err)
	}
	if err := initCheckDir(logpath); err != nil {
		return debugerror.Errorf("logs: %s", err)
	}

	return nil
}

func (r *FSRepo) SetConfig(conf *config.Config) error {
	configFilename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	if err := WriteConfigFile(configFilename, conf); err != nil {
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

// initCheckDir ensures the directory exists and is writable
func initCheckDir(path string) error {
	// Construct the path if missing
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	// Check the directory is writeable
	if f, err := os.Create(filepath.Join(path, "._check_writeable")); err == nil {
		os.Remove(f.Name())
	} else {
		return debugerror.New("'" + path + "' is not writeable")
	}
	return nil
}
