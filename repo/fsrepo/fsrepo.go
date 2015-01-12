package fsrepo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/debugerror"
)

type FSRepo struct {
	state  state
	path   string
	config *config.Config
}

func At(path string) *FSRepo {
	return &FSRepo{
		path:  path,
		state: unopened, // explicitly set for clarity
	}
}

func Init(path string, conf *config.Config) error {
	if IsInitialized(path) {
		return nil
	}
	configFilename, err := config.Filename(path)
	if err != nil {
		return err
	}
	if err := writeConfigFile(configFilename, conf); err != nil {
		return err
	}
	return nil
}

// Open returns an error if the repo is not initialized.
func (r *FSRepo) Open() error {
	if r.state != unopened {
		return debugerror.Errorf("repo is %s", r.state)
	}
	if !IsInitialized(r.path) {
		return debugerror.New("repo is not initialized")
	}
	// check repo path, then check all constituent parts.
	// TODO acquire repo lock
	// TODO if err := initCheckDir(logpath); err != nil { // }
	if err := initCheckDir(r.path); err != nil {
		return err
	}

	configFilename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	conf, err := Load(configFilename)
	if err != nil {
		return err
	}
	r.config = conf

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

	r.state = opened
	return nil
}

func (r *FSRepo) Config() *config.Config {
	if r.state != opened {
		panic(fmt.Sprintln("repo is", r.state))
	}
	return r.config
}

func (r *FSRepo) SetConfig(conf *config.Config) error {
	if r.state != opened {
		panic(fmt.Sprintln("repo is", r.state))
	}
	configFilename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	if err := writeConfigFile(configFilename, conf); err != nil {
		return err
	}
	*r.config = *conf // copy so caller cannot modify the private config
	return nil
}

func (r *FSRepo) Close() error {
	if r.state != opened {
		return debugerror.Errorf("repo is %s", r.state)
	}
	return nil // TODO release repo lock
}

var _ io.Closer = &FSRepo{}

// IsInitialized returns true if the repo is initialized at provided |path|.
func IsInitialized(path string) bool {
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
