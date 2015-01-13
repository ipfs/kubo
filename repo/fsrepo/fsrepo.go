package fsrepo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	repo "github.com/jbenet/go-ipfs/repo"
	common "github.com/jbenet/go-ipfs/repo/common"
	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

// FSRepo represents an IPFS FileSystem Repo
type FSRepo struct {
	state  state
	path   string
	config *config.Config
}

// At returns a handle to an FSRepo at the provided |path|.
func At(path string) *FSRepo {
	return &FSRepo{
		path:  path,
		state: unopened, // explicitly set for clarity
	}
}

// Init initializes a new FSRepo at the given path with the provided config.
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
		return debugerror.New("ipfs not initialized, please run 'ipfs init'")
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
	conf, err := load(configFilename)
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

// Config returns the FSRepo's config. Result is undefined if the Repo is not
// Open.
func (r *FSRepo) Config() *config.Config {
	if r.state != opened {
		panic(fmt.Sprintln("repo is", r.state))
	}
	return r.config
}

// SetConfig updates the FSRepo's config.
func (r *FSRepo) SetConfig(updated *config.Config) error {
	if r.state != opened {
		panic(fmt.Sprintln("repo is", r.state))
	}
	configFilename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	// to avoid clobbering user-provided keys, must read the config from disk
	// as a map, write the updated struct values to the map and write the map
	// to disk.
	var mapconf map[string]interface{}
	if err := readConfigFile(configFilename, &mapconf); err != nil {
		return err
	}
	m, err := config.ToMap(updated)
	if err != nil {
		return err
	}
	for k, v := range m {
		mapconf[k] = v
	}
	if err := writeConfigFile(configFilename, mapconf); err != nil {
		return err
	}
	*r.config = *updated // copy so caller cannot modify this private config
	return nil
}

// GetConfigKey retrieves only the value of a particular key.
func (r *FSRepo) GetConfigKey(key string) (interface{}, error) {
	if r.state != opened {
		return nil, debugerror.Errorf("repo is %s", r.state)
	}
	filename, err := config.Filename(r.path)
	if err != nil {
		return nil, err
	}
	var cfg map[string]interface{}
	if err := readConfigFile(filename, &cfg); err != nil {
		return nil, err
	}
	return common.MapGetKV(cfg, key)
}

// SetConfigKey writes the value of a particular key.
func (r *FSRepo) SetConfigKey(key string, value interface{}) error {
	if r.state != opened {
		return debugerror.Errorf("repo is %s", r.state)
	}
	filename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	var mapconf map[string]interface{}
	if err := readConfigFile(filename, &mapconf); err != nil {
		return err
	}
	if err := common.MapSetKV(mapconf, key, value); err != nil {
		return err
	}
	if err := writeConfigFile(filename, mapconf); err != nil {
		return err
	}
	conf, err := config.FromMap(mapconf)
	if err != nil {
		return err
	}
	return r.SetConfig(conf)
}

// Close closes the FSRepo, releasing held resources.
func (r *FSRepo) Close() error {
	if r.state != opened {
		return debugerror.Errorf("repo is %s", r.state)
	}
	return nil // TODO release repo lock
}

var _ io.Closer = &FSRepo{}
var _ repo.Interface = &FSRepo{}

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
