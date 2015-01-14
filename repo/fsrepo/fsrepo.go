package fsrepo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"

	repo "github.com/jbenet/go-ipfs/repo"
	config "github.com/jbenet/go-ipfs/repo/config"
	lockfile "github.com/jbenet/go-ipfs/repo/fsrepo/lock"
	opener "github.com/jbenet/go-ipfs/repo/fsrepo/opener"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

var (

	// packageLock must be held to while performing any operation that modifies an
	// FSRepo's state field. This includes Init, Open, Close, and Remove.
	packageLock sync.Mutex // protects openerCounter and lockfiles
	// lockfiles holds references to the Closers that ensure that repos are
	// only accessed by one process at a time.
	lockfiles map[string]io.Closer
	// openerCounter prevents the fsrepo from being removed while there exist open
	// FSRepo handles. It also ensures that the Init is atomic.
	//
	// packageLock also protects numOpenedRepos
	//
	// If an operation is used when repo is Open and the operation does not
	// change the repo's state, the package lock does not need to be acquired.
	openerCounter *opener.Counter
)

func init() {
	openerCounter = opener.NewCounter()
	lockfiles = make(map[string]io.Closer)
}

// FSRepo represents an IPFS FileSystem Repo. It is safe for use by multiple
// callers.
type FSRepo struct {
	// state is the FSRepo's state (unopened, opened, closed)
	state state
	// path is the file-system path
	path string
	// config is loaded when FSRepo is opened and kept up to date when the
	// FSRepo is modified.
	// TODO test
	configComponent configComponent
}

type component interface {
	Open() error
	io.Closer
}
type componentInitializationChecker func(path string) bool

// At returns a handle to an FSRepo at the provided |path|.
func At(repoPath string) *FSRepo {
	// This method must not have side-effects.
	return &FSRepo{
		path:            path.Clean(repoPath),
		configComponent: makeConfigComponent(repoPath),
		state:           unopened, // explicitly set for clarity
	}
}

func ConfigAt(repoPath string) (*config.Config, error) {

	// packageLock must be held to ensure that the Read is atomic.
	packageLock.Lock()
	defer packageLock.Unlock()

	configFilename, err := config.Filename(repoPath)
	if err != nil {
		return nil, err
	}
	return load(configFilename)
}

// Init initializes a new FSRepo at the given path with the provided config.
// TODO add support for custom datastores.
func Init(path string, conf *config.Config) error {

	// packageLock must be held to ensure that the repo is not initialized more
	// than once.
	packageLock.Lock()
	defer packageLock.Unlock()

	if isInitializedUnsynced(path) {
		return nil
	}
	if err := initConfigComponent(path, conf); err != nil {
		return err
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

	if openerCounter.NumOpeners(repoPath) != 0 {
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
	return lockfile.Locked(repoPath) && openerCounter.NumOpeners(repoPath) == 0
}

// Open returns an error if the repo is not initialized.
func (r *FSRepo) Open() error {

	// packageLock must be held to make sure that the repo is not destroyed by
	// another caller. It must not be released until initialization is complete
	// and the number of openers is incremeneted.
	packageLock.Lock()
	defer packageLock.Unlock()

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

	for _, opener := range r.components() {
		if err := opener.Open(); err != nil {
			return err
		}
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

	return transitionToOpened(r)
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
	return transitionToClosed(r)
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
	return r.configComponent.Config()
}

// SetConfig updates the FSRepo's config.
func (r *FSRepo) SetConfig(updated *config.Config) error {

	// packageLock is held to provide thread-safety.
	packageLock.Lock()
	defer packageLock.Unlock()

	return r.configComponent.SetConfig(updated)
}

// GetConfigKey retrieves only the value of a particular key.
func (r *FSRepo) GetConfigKey(key string) (interface{}, error) {
	packageLock.Lock()
	defer packageLock.Unlock()

	if r.state != opened {
		return nil, debugerror.Errorf("repo is %s", r.state)
	}
	return r.configComponent.GetConfigKey(key)
}

// SetConfigKey writes the value of a particular key.
func (r *FSRepo) SetConfigKey(key string, value interface{}) error {
	packageLock.Lock()
	defer packageLock.Unlock()

	if r.state != opened {
		return debugerror.Errorf("repo is %s", r.state)
	}
	return r.configComponent.SetConfigKey(key, value)
}

var _ io.Closer = &FSRepo{}
var _ repo.Repo = &FSRepo{}

// IsInitialized returns true if the repo is initialized at provided |path|.
func IsInitialized(path string) bool {
	// packageLock is held to ensure that another caller doesn't attempt to
	// Init or Remove the repo while this call is in progress.
	packageLock.Lock()
	defer packageLock.Unlock()

	// componentInitCheckers are functions that indicate whether the component
	// is isInitialized
	var componentInitCheckers = []componentInitializationChecker{
		configComponentIsInitialized,
		// TODO add datastore component initialization checker
	}
	for _, isInitialized := range componentInitCheckers {
		if !isInitialized(path) {
			return false
		}
	}
	return true
}

// private methods below this point. NB: packageLock must held by caller.

// isInitializedUnsynced reports whether the repo is initialized. Caller must
// hold openerCounter lock.
func isInitializedUnsynced(path string) bool {
	return configComponentIsInitialized(path)
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
// the package mutex.
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
// hold the package mutex.
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

// components returns the FSRepo's constituent components
func (r *FSRepo) components() []component {
	return []component{
		&r.configComponent,
		// TODO add datastore
	}
}
