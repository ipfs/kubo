package fsrepo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	repo "github.com/jbenet/go-ipfs/repo"
	common "github.com/jbenet/go-ipfs/repo/common"
	config "github.com/jbenet/go-ipfs/repo/config"
	lockfile "github.com/jbenet/go-ipfs/repo/fsrepo/lock"
	opener "github.com/jbenet/go-ipfs/repo/fsrepo/opener"
	util "github.com/jbenet/go-ipfs/util"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

var (
	// openerCounter prevents the fsrepo from being removed while there exist open
	// FSRepo handles. It also ensures that the Init is atomic.
	//
	// packageLock also protects numOpenedRepos
	//
	// If an operation is used when repo is Open and the operation does not
	// change the repo's state, the package lock does not need to be acquired.
	openerCounter *opener.Counter

	lockfiles map[string]io.Closer
)

func init() {
	openerCounter = opener.NewCounter()
	lockfiles = make(map[string]io.Closer)
}

// FSRepo represents an IPFS FileSystem Repo. It is not thread-safe.
type FSRepo struct {
	state  state
	path   string
	config *config.Config
}

// At returns a handle to an FSRepo at the provided |path|.
func At(repoPath string) *FSRepo {
	// This method must not have side-effects.
	return &FSRepo{
		path:  path.Clean(repoPath),
		state: unopened, // explicitly set for clarity
	}
}

func ConfigAt(repoPath string) (*config.Config, error) {
	configFilename, err := config.Filename(repoPath)
	if err != nil {
		return nil, err
	}
	return load(configFilename)
}

// Init initializes a new FSRepo at the given path with the provided config.
func Init(path string, conf *config.Config) error {
	openerCounter.Lock() // lock must be held to ensure atomicity (prevent Removal)
	defer openerCounter.Unlock()

	if isInitializedUnsynced(path) {
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

// Remove recursively removes the FSRepo at |path|.
func Remove(path string) error {
	openerCounter.Lock()
	defer openerCounter.Unlock()
	if openerCounter.NumOpeners(path) != 0 {
		return errors.New("repo in use")
	}
	return os.RemoveAll(path)
}

// LockedByOtherProcess returns true if the FSRepo is locked by another
// process. If true, then the repo cannot be opened by this process.
func LockedByOtherProcess(repoPath string) bool {
	openerCounter.Lock()
	defer openerCounter.Unlock()
	// NB: the lock is only held when repos are Open
	return lockfile.Locked(repoPath) && openerCounter.NumOpeners(repoPath) == 0
}

// Open returns an error if the repo is not initialized.
func (r *FSRepo) Open() error {
	openerCounter.Lock()
	defer openerCounter.Unlock()
	if r.state != unopened {
		return debugerror.Errorf("repo is %s", r.state)
	}
	if !isInitializedUnsynced(r.path) {
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

	return transitionToOpened(r)
}

// Config returns the FSRepo's config. This method must not be called if the
// repo is not open.
//
// Result when not Open is undefined. The method may panic if it pleases.
func (r *FSRepo) Config() *config.Config {
	// no lock necessary because repo is either Open (and thus protected from
	// Removal) or has no side-effect
	if r.state != opened {
		panic(fmt.Sprintln("repo is", r.state))
	}
	return r.config
}

// SetConfig updates the FSRepo's config.
func (r *FSRepo) SetConfig(updated *config.Config) error {
	// no lock required because repo should be Open
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
	// no lock required because repo should be Open
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
	openerCounter.Lock()
	defer openerCounter.Unlock()
	if r.state != opened {
		return debugerror.Errorf("repo is %s", r.state)
	}
	return transitionToClosed(r)
}

var _ io.Closer = &FSRepo{}
var _ repo.Repo = &FSRepo{}

// IsInitialized returns true if the repo is initialized at provided |path|.
func IsInitialized(path string) bool {
	openerCounter.Lock()
	defer openerCounter.Unlock()
	return isInitializedUnsynced(path)
}

// isInitializedUnsynced reports whether the repo is initialized. Caller must
// hold openerCounter lock.
func isInitializedUnsynced(path string) bool {
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

// transitionToOpened manages the state transition to |opened|. Caller must hold
// openerCounter lock.
func transitionToOpened(r *FSRepo) error {
	r.state = opened
	if countBefore := openerCounter.NumOpeners(r.path); countBefore == 0 { // #first
		closer, err := lockfile.Lock(r.path)
		if err != nil {
			return err
		}
		lockfiles[r.path] = closer
	}
	return openerCounter.AddOpener(r.path)
}

// transitionToClosed manages the state transition to |closed|. Caller must
// hold openerCounter lock.
func transitionToClosed(r *FSRepo) error {
	r.state = closed
	if err := openerCounter.RemoveOpener(r.path); err != nil {
		return err
	}
	if countAfter := openerCounter.NumOpeners(r.path); countAfter == 0 {
		closer, ok := lockfiles[r.path]
		if !ok {
			return errors.New("package error: lockfile is not held")
		}
		if err := closer.Close(); err != nil {
			return err
		}
		delete(lockfiles, r.path)
	}
	return nil
}
