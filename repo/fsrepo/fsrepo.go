package fsrepo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"sync"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	repo "github.com/jbenet/go-ipfs/repo"
	"github.com/jbenet/go-ipfs/repo/common"
	config "github.com/jbenet/go-ipfs/repo/config"
	component "github.com/jbenet/go-ipfs/repo/fsrepo/component"
	counter "github.com/jbenet/go-ipfs/repo/fsrepo/counter"
	lockfile "github.com/jbenet/go-ipfs/repo/fsrepo/lock"
	serialize "github.com/jbenet/go-ipfs/repo/fsrepo/serialize"
	dir "github.com/jbenet/go-ipfs/thirdparty/dir"
	u "github.com/jbenet/go-ipfs/util"
	util "github.com/jbenet/go-ipfs/util"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

var (

	// packageLock must be held to while performing any operation that modifies an
	// FSRepo's state field. This includes Init, Open, Close, and Remove.
	packageLock sync.Mutex // protects openersCounter and lockfiles
	// lockfiles holds references to the Closers that ensure that repos are
	// only accessed by one process at a time.
	lockfiles map[string]io.Closer
	// openersCounter prevents the fsrepo from being removed while there exist open
	// FSRepo handles. It also ensures that the Init is atomic.
	//
	// packageLock also protects numOpenedRepos
	//
	// If an operation is used when repo is Open and the operation does not
	// change the repo's state, the package lock does not need to be acquired.
	openersCounter *counter.Openers
)

func init() {
	openersCounter = counter.NewOpenersCounter()
	lockfiles = make(map[string]io.Closer)
}

// FSRepo represents an IPFS FileSystem Repo. It is safe for use by multiple
// callers.
type FSRepo struct {
	// state is the FSRepo's state (unopened, opened, closed)
	state state
	// path is the file-system path
	path string
	// config is set on Open, guarded by packageLock
	config *config.Config

	// TODO test
	datastoreComponent component.DatastoreComponent
	eventlogComponent  component.EventlogComponent
}

var _ repo.Repo = (*FSRepo)(nil)

type componentBuilder struct {
	Init          component.Initializer
	IsInitialized component.InitializationChecker
	OpenHandler   func(*FSRepo) error
}

// At returns a handle to an FSRepo at the provided |path|.
func At(repoPath string) *FSRepo {
	// This method must not have side-effects.
	return &FSRepo{
		path:  path.Clean(repoPath),
		state: unopened, // explicitly set for clarity
	}
}

// ConfigAt returns an error if the FSRepo at the given path is not
// initialized. This function allows callers to read the config file even when
// another process is running and holding the lock.
func ConfigAt(repoPath string) (*config.Config, error) {

	// packageLock must be held to ensure that the Read is atomic.
	packageLock.Lock()
	defer packageLock.Unlock()

	configFilename, err := config.Filename(repoPath)
	if err != nil {
		return nil, err
	}
	return serialize.Load(configFilename)
}

// configIsInitialized returns true if the repo is initialized at
// provided |path|.
func configIsInitialized(path string) bool {
	configFilename, err := config.Filename(path)
	if err != nil {
		return false
	}
	if !util.FileExists(configFilename) {
		return false
	}
	return true
}

func initConfig(path string, conf *config.Config) error {
	if configIsInitialized(path) {
		return nil
	}
	configFilename, err := config.Filename(path)
	if err != nil {
		return err
	}
	// initialization is the one time when it's okay to write to the config
	// without reading the config from disk and merging any user-provided keys
	// that may exist.
	if err := serialize.WriteConfigFile(configFilename, conf); err != nil {
		return err
	}
	return nil
}

// Init initializes a new FSRepo at the given path with the provided config.
// TODO add support for custom datastores.
func Init(repoPath string, conf *config.Config) error {

	// packageLock must be held to ensure that the repo is not initialized more
	// than once.
	packageLock.Lock()
	defer packageLock.Unlock()

	if isInitializedUnsynced(repoPath) {
		return nil
	}

	if err := initConfig(repoPath, conf); err != nil {
		return err
	}

	for _, b := range componentBuilders() {
		if err := b.Init(repoPath, conf); err != nil {
			return err
		}
	}
	return nil
}

// Remove recursively removes the FSRepo at |path|.
func Remove(repoPath string) error {
	repoPath = path.Clean(repoPath)

	// packageLock must be held to ensure that the repo is not removed while
	// being accessed by others.
	packageLock.Lock()
	defer packageLock.Unlock()

	if openersCounter.NumOpeners(repoPath) != 0 {
		return errors.New("repo in use")
	}
	return os.RemoveAll(repoPath)
}

// LockedByOtherProcess returns true if the FSRepo is locked by another
// process. If true, then the repo cannot be opened by this process.
func LockedByOtherProcess(repoPath string) bool {
	repoPath = path.Clean(repoPath)

	// packageLock must be held to check the number of openers.
	packageLock.Lock()
	defer packageLock.Unlock()

	// NB: the lock is only held when repos are Open
	return lockfile.Locked(repoPath) && openersCounter.NumOpeners(repoPath) == 0
}

// openConfig returns an error if the config file is not present.
func (r *FSRepo) openConfig() error {
	configFilename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	conf, err := serialize.Load(configFilename)
	if err != nil {
		return err
	}
	r.config = conf
	return nil
}

// Open returns an error if the repo is not initialized.
func (r *FSRepo) Open() error {

	// packageLock must be held to make sure that the repo is not destroyed by
	// another caller. It must not be released until initialization is complete
	// and the number of openers is incremeneted.
	packageLock.Lock()
	defer packageLock.Unlock()

	expPath, err := u.TildeExpansion(r.path)
	if err != nil {
		return err
	}
	r.path = expPath

	if r.state != unopened {
		return debugerror.Errorf("repo is %s", r.state)
	}
	if !isInitializedUnsynced(r.path) {
		return debugerror.New("ipfs not initialized, please run 'ipfs init'")
	}
	// check repo path, then check all constituent parts.
	// TODO acquire repo lock
	// TODO if err := initCheckDir(logpath); err != nil { // }
	if err := dir.Writable(r.path); err != nil {
		return err
	}

	if err := r.openConfig(); err != nil {
		return err
	}

	for _, b := range componentBuilders() {
		if err := b.OpenHandler(r); err != nil {
			return err
		}
	}

	return r.transitionToOpened()
}

// Close closes the FSRepo, releasing held resources.
func (r *FSRepo) Close() error {
	packageLock.Lock()
	defer packageLock.Unlock()

	if r.state != opened {
		return debugerror.Errorf("repo is %s", r.state)
	}

	for _, closer := range r.components() {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return r.transitionToClosed()
}

// Config returns the FSRepo's config. This method must not be called if the
// repo is not open.
//
// Result when not Open is undefined. The method may panic if it pleases.
func (r *FSRepo) Config() *config.Config {

	// It is not necessary to hold the package lock since the repo is in an
	// opened state. The package lock is _not_ meant to ensure that the repo is
	// thread-safe. The package lock is only meant to guard againt removal and
	// coordinate the lockfile. However, we provide thread-safety to keep
	// things simple.
	packageLock.Lock()
	defer packageLock.Unlock()

	if r.state != opened {
		panic(fmt.Sprintln("repo is", r.state))
	}
	return r.config
}

// setConfigUnsynced is for private use.
func (r *FSRepo) setConfigUnsynced(updated *config.Config) error {
	configFilename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	// to avoid clobbering user-provided keys, must read the config from disk
	// as a map, write the updated struct values to the map and write the map
	// to disk.
	var mapconf map[string]interface{}
	if err := serialize.ReadConfigFile(configFilename, &mapconf); err != nil {
		return err
	}
	m, err := config.ToMap(updated)
	if err != nil {
		return err
	}
	for k, v := range m {
		mapconf[k] = v
	}
	if err := serialize.WriteConfigFile(configFilename, mapconf); err != nil {
		return err
	}
	*r.config = *updated // copy so caller cannot modify this private config
	return nil
}

// SetConfig updates the FSRepo's config.
func (r *FSRepo) SetConfig(updated *config.Config) error {

	// packageLock is held to provide thread-safety.
	packageLock.Lock()
	defer packageLock.Unlock()

	return r.setConfigUnsynced(updated)
}

// GetConfigKey retrieves only the value of a particular key.
func (r *FSRepo) GetConfigKey(key string) (interface{}, error) {
	packageLock.Lock()
	defer packageLock.Unlock()

	if r.state != opened {
		return nil, debugerror.Errorf("repo is %s", r.state)
	}

	filename, err := config.Filename(r.path)
	if err != nil {
		return nil, err
	}
	var cfg map[string]interface{}
	if err := serialize.ReadConfigFile(filename, &cfg); err != nil {
		return nil, err
	}
	return common.MapGetKV(cfg, key)
}

// SetConfigKey writes the value of a particular key.
func (r *FSRepo) SetConfigKey(key string, value interface{}) error {
	packageLock.Lock()
	defer packageLock.Unlock()

	if r.state != opened {
		return debugerror.Errorf("repo is %s", r.state)
	}

	filename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	switch v := value.(type) {
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			value = i
		}
	}
	var mapconf map[string]interface{}
	if err := serialize.ReadConfigFile(filename, &mapconf); err != nil {
		return err
	}
	if err := common.MapSetKV(mapconf, key, value); err != nil {
		return err
	}
	conf, err := config.FromMap(mapconf)
	if err != nil {
		return err
	}
	if err := serialize.WriteConfigFile(filename, mapconf); err != nil {
		return err
	}
	return r.setConfigUnsynced(conf) // TODO roll this into this method
}

// Datastore returns a repo-owned datastore. If FSRepo is Closed, return value
// is undefined.
func (r *FSRepo) Datastore() ds.ThreadSafeDatastore {
	packageLock.Lock()
	d := r.datastoreComponent.Datastore()
	packageLock.Unlock()
	return d
}

var _ io.Closer = &FSRepo{}
var _ repo.Repo = &FSRepo{}

// IsInitialized returns true if the repo is initialized at provided |path|.
func IsInitialized(path string) bool {
	// packageLock is held to ensure that another caller doesn't attempt to
	// Init or Remove the repo while this call is in progress.
	packageLock.Lock()
	defer packageLock.Unlock()

	return isInitializedUnsynced(path)
}

// private methods below this point. NB: packageLock must held by caller.

// isInitializedUnsynced reports whether the repo is initialized. Caller must
// hold the packageLock.
func isInitializedUnsynced(path string) bool {
	if !configIsInitialized(path) {
		return false
	}
	for _, b := range componentBuilders() {
		if !b.IsInitialized(path) {
			return false
		}
	}
	return true
}

// transitionToOpened manages the state transition to |opened|. Caller must hold
// the package mutex.
func (r *FSRepo) transitionToOpened() error {
	r.state = opened
	if countBefore := openersCounter.NumOpeners(r.path); countBefore == 0 { // #first
		closer, err := lockfile.Lock(r.path)
		if err != nil {
			return err
		}
		lockfiles[r.path] = closer
	}
	return openersCounter.AddOpener(r.path)
}

// transitionToClosed manages the state transition to |closed|. Caller must
// hold the package mutex.
func (r *FSRepo) transitionToClosed() error {
	r.state = closed
	if err := openersCounter.RemoveOpener(r.path); err != nil {
		return err
	}
	if countAfter := openersCounter.NumOpeners(r.path); countAfter == 0 {
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

// components returns the FSRepo's constituent components
func (r *FSRepo) components() []component.Component {
	return []component.Component{
		&r.datastoreComponent,
	}
}

func componentBuilders() []componentBuilder {
	return []componentBuilder{

		// DatastoreComponent
		componentBuilder{
			Init:          component.InitDatastoreComponent,
			IsInitialized: component.DatastoreComponentIsInitialized,
			OpenHandler: func(r *FSRepo) error {
				c := component.DatastoreComponent{}
				c.SetPath(r.path)
				if err := c.Open(r.config); err != nil {
					return err
				}
				r.datastoreComponent = c
				return nil
			},
		},

		// EventlogComponent
		componentBuilder{
			Init:          component.InitEventlogComponent,
			IsInitialized: component.EventlogComponentIsInitialized,
			OpenHandler: func(r *FSRepo) error {
				c := component.EventlogComponent{}
				c.SetPath(r.path)
				if err := c.Open(r.config); err != nil {
					return err
				}
				r.eventlogComponent = c
				return nil
			},
		},
	}
}
