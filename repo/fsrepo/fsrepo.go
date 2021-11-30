package fsrepo

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	filestore "github.com/ipfs/go-filestore"
	keystore "github.com/ipfs/go-ipfs-keystore"
	repo "github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/common"
	dir "github.com/ipfs/go-ipfs/thirdparty/dir"

	ds "github.com/ipfs/go-datastore"
	measure "github.com/ipfs/go-ds-measure"
	lockfile "github.com/ipfs/go-fs-lock"
	config "github.com/ipfs/go-ipfs-config"
	serialize "github.com/ipfs/go-ipfs-config/serialize"
	util "github.com/ipfs/go-ipfs-util"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	logging "github.com/ipfs/go-log"
	homedir "github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
)

// LockFile is the filename of the repo lock, relative to config dir
// TODO rename repo lock and hide name
const LockFile = "repo.lock"

var log = logging.Logger("fsrepo")

// version number that we are currently expecting to see
var RepoVersion = 11

var migrationInstructions = `See https://github.com/ipfs/fs-repo-migrations/blob/master/run.md
Sorry for the inconvenience. In the future, these will run automatically.`

var programTooLowMessage = `Your programs version (%d) is lower than your repos (%d).
Please update ipfs to a version that supports the existing repo, or run
a migration in reverse.

See https://github.com/ipfs/fs-repo-migrations/blob/master/run.md for details.`

var (
	ErrNoVersion     = errors.New("no version file found, please run 0-to-1 migration tool.\n" + migrationInstructions)
	ErrOldRepo       = errors.New("ipfs repo found in old '~/.go-ipfs' location, please run migration tool.\n" + migrationInstructions)
	ErrNeedMigration = errors.New("ipfs repo needs migration")
)

type NoRepoError struct {
	Path string
}

var _ error = NoRepoError{}

func (err NoRepoError) Error() string {
	return fmt.Sprintf("no IPFS repo found in %s.\nplease run: 'ipfs init'", err.Path)
}

const apiFile = "api"
const swarmKeyFile = "swarm.key"

const specFn = "datastore_spec"

var (

	// packageLock must be held to while performing any operation that modifies an
	// FSRepo's state field. This includes Init, Open, Close, and Remove.
	packageLock sync.Mutex

	// onlyOne keeps track of open FSRepo instances.
	//
	// TODO: once command Context / Repo integration is cleaned up,
	// this can be removed. Right now, this makes ConfigCmd.Run
	// function try to open the repo twice:
	//
	//     $ ipfs daemon &
	//     $ ipfs config foo
	//
	// The reason for the above is that in standalone mode without the
	// daemon, `ipfs config` tries to save work by not building the
	// full IpfsNode, but accessing the Repo directly.
	onlyOne repo.OnlyOne
)

// FSRepo represents an IPFS FileSystem Repo. It is safe for use by multiple
// callers.
type FSRepo struct {
	// has Close been called already
	closed bool
	// path is the file-system path
	path string
	// lockfile is the file system lock to prevent others from opening
	// the same fsrepo path concurrently
	lockfile io.Closer
	config   *config.Config
	ds       repo.Datastore
	keystore keystore.Keystore
	filemgr  *filestore.FileManager
}

var _ repo.Repo = (*FSRepo)(nil)

// Open the FSRepo at path. Returns an error if the repo is not
// initialized.
func Open(repoPath string) (repo.Repo, error) {
	fn := func() (repo.Repo, error) {
		return open(repoPath)
	}
	return onlyOne.Open(repoPath, fn)
}

func open(repoPath string) (repo.Repo, error) {
	packageLock.Lock()
	defer packageLock.Unlock()

	r, err := newFSRepo(repoPath)
	if err != nil {
		return nil, err
	}

	// Check if its initialized
	if err := checkInitialized(r.path); err != nil {
		return nil, err
	}

	r.lockfile, err = lockfile.Lock(r.path, LockFile)
	if err != nil {
		return nil, err
	}
	keepLocked := false
	defer func() {
		// unlock on error, leave it locked on success
		if !keepLocked {
			r.lockfile.Close()
		}
	}()

	// Check version, and error out if not matching
	ver, err := migrations.RepoVersion(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoVersion
		}
		return nil, err
	}

	if RepoVersion > ver {
		return nil, ErrNeedMigration
	} else if ver > RepoVersion {
		// program version too low for existing repo
		return nil, fmt.Errorf(programTooLowMessage, RepoVersion, ver)
	}

	// check repo path, then check all constituent parts.
	if err := dir.Writable(r.path); err != nil {
		return nil, err
	}

	if err := r.openConfig(); err != nil {
		return nil, err
	}

	if err := r.openDatastore(); err != nil {
		return nil, err
	}

	if err := r.openKeystore(); err != nil {
		return nil, err
	}

	if r.config.Experimental.FilestoreEnabled || r.config.Experimental.UrlstoreEnabled {
		r.filemgr = filestore.NewFileManager(r.ds, filepath.Dir(r.path))
		r.filemgr.AllowFiles = r.config.Experimental.FilestoreEnabled
		r.filemgr.AllowUrls = r.config.Experimental.UrlstoreEnabled
	}

	keepLocked = true
	return r, nil
}

func newFSRepo(rpath string) (*FSRepo, error) {
	expPath, err := homedir.Expand(filepath.Clean(rpath))
	if err != nil {
		return nil, err
	}

	return &FSRepo{path: expPath}, nil
}

func checkInitialized(path string) error {
	if !isInitializedUnsynced(path) {
		alt := strings.Replace(path, ".ipfs", ".go-ipfs", 1)
		if isInitializedUnsynced(alt) {
			return ErrOldRepo
		}
		return NoRepoError{Path: path}
	}
	return nil
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

func initSpec(path string, conf map[string]interface{}) error {
	fn, err := config.Path(path, specFn)
	if err != nil {
		return err
	}

	if util.FileExists(fn) {
		return nil
	}

	dsc, err := AnyDatastoreConfig(conf)
	if err != nil {
		return err
	}
	bytes := dsc.DiskSpec().Bytes()

	return ioutil.WriteFile(fn, bytes, 0600)
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

	if err := initSpec(repoPath, conf.Datastore.Spec); err != nil {
		return err
	}

	if err := migrations.WriteRepoVersion(repoPath, RepoVersion); err != nil {
		return err
	}

	return nil
}

// LockedByOtherProcess returns true if the FSRepo is locked by another
// process. If true, then the repo cannot be opened by this process.
func LockedByOtherProcess(repoPath string) (bool, error) {
	repoPath = filepath.Clean(repoPath)
	locked, err := lockfile.Locked(repoPath, LockFile)
	if locked {
		log.Debugf("(%t)<->Lock is held at %s", locked, repoPath)
	}
	return locked, err
}

// APIAddr returns the registered API addr, according to the api file
// in the fsrepo. This is a concurrent operation, meaning that any
// process may read this file. modifying this file, therefore, should
// use "mv" to replace the whole file and avoid interleaved read/writes.
func APIAddr(repoPath string) (ma.Multiaddr, error) {
	repoPath = filepath.Clean(repoPath)
	apiFilePath := filepath.Join(repoPath, apiFile)

	// if there is no file, assume there is no api addr.
	f, err := os.Open(apiFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, repo.ErrApiNotRunning
		}
		return nil, err
	}
	defer f.Close()

	// read up to 2048 bytes. io.ReadAll is a vulnerability, as
	// someone could hose the process by putting a massive file there.
	//
	// NOTE(@stebalien): @jbenet probably wasn't thinking straight when he
	// wrote that comment but I'm leaving the limit here in case there was
	// some hidden wisdom. However, I'm fixing it such that:
	// 1. We don't read too little.
	// 2. We don't truncate and succeed.
	buf, err := ioutil.ReadAll(io.LimitReader(f, 2048))
	if err != nil {
		return nil, err
	}
	if len(buf) == 2048 {
		return nil, fmt.Errorf("API file too large, must be <2048 bytes long: %s", apiFilePath)
	}

	s := string(buf)
	s = strings.TrimSpace(s)
	return ma.NewMultiaddr(s)
}

func (r *FSRepo) Keystore() keystore.Keystore {
	return r.keystore
}

func (r *FSRepo) Path() string {
	return r.path
}

// SetAPIAddr writes the API Addr to the /api file.
func (r *FSRepo) SetAPIAddr(addr ma.Multiaddr) error {
	// Create a temp file to write the address, so that we don't leave empty file when the
	// program crashes after creating the file.
	f, err := os.Create(filepath.Join(r.path, "."+apiFile+".tmp"))
	if err != nil {
		return err
	}

	if _, err = f.WriteString(addr.String()); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}

	// Atomically rename the temp file to the correct file name.
	if err = os.Rename(filepath.Join(r.path, "."+apiFile+".tmp"), filepath.Join(r.path,
		apiFile)); err == nil {
		return nil
	}
	// Remove the temp file when rename return error
	if err1 := os.Remove(filepath.Join(r.path, "."+apiFile+".tmp")); err1 != nil {
		return fmt.Errorf("File Rename error: %s, File remove error: %s", err.Error(),
			err1.Error())
	}
	return err
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

func (r *FSRepo) openKeystore() error {
	ksp := filepath.Join(r.path, "keystore")
	ks, err := keystore.NewFSKeystore(ksp)
	if err != nil {
		return err
	}

	r.keystore = ks

	return nil
}

// openDatastore returns an error if the config file is not present.
func (r *FSRepo) openDatastore() error {
	if r.config.Datastore.Type != "" || r.config.Datastore.Path != "" {
		return fmt.Errorf("old style datatstore config detected")
	} else if r.config.Datastore.Spec == nil {
		return fmt.Errorf("required Datastore.Spec entry missing from config file")
	}
	if r.config.Datastore.NoSync {
		log.Warn("NoSync is now deprecated in favor of datastore specific settings. If you want to disable fsync on flatfs set 'sync' to false. See https://github.com/ipfs/go-ipfs/blob/master/docs/datastores.md#flatfs.")
	}

	dsc, err := AnyDatastoreConfig(r.config.Datastore.Spec)
	if err != nil {
		return err
	}
	spec := dsc.DiskSpec()

	oldSpec, err := r.readSpec()
	if err != nil {
		return err
	}
	if oldSpec != spec.String() {
		return fmt.Errorf("datastore configuration of '%s' does not match what is on disk '%s'",
			oldSpec, spec.String())
	}

	d, err := dsc.Create(r.path)
	if err != nil {
		return err
	}
	r.ds = d

	// Wrap it with metrics gathering
	prefix := "ipfs.fsrepo.datastore"
	r.ds = measure.New(prefix, r.ds)

	return nil
}

func (r *FSRepo) readSpec() (string, error) {
	fn, err := config.Path(r.path, specFn)
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// Close closes the FSRepo, releasing held resources.
func (r *FSRepo) Close() error {
	packageLock.Lock()
	defer packageLock.Unlock()

	if r.closed {
		return errors.New("repo is closed")
	}

	err := os.Remove(filepath.Join(r.path, apiFile))
	if err != nil && !os.IsNotExist(err) {
		log.Warn("error removing api file: ", err)
	}

	if err := r.ds.Close(); err != nil {
		return err
	}

	// This code existed in the previous versions, but
	// EventlogComponent.Close was never called. Preserving here
	// pending further discussion.
	//
	// TODO It isn't part of the current contract, but callers may like for us
	// to disable logging once the component is closed.
	// logging.Configure(logging.Output(os.Stderr))

	r.closed = true
	return r.lockfile.Close()
}

// Config the current config. This function DOES NOT copy the config. The caller
// MUST NOT modify it without first calling `Clone`.
//
// Result when not Open is undefined. The method may panic if it pleases.
func (r *FSRepo) Config() (*config.Config, error) {
	// It is not necessary to hold the package lock since the repo is in an
	// opened state. The package lock is _not_ meant to ensure that the repo is
	// thread-safe. The package lock is only meant to guard against removal and
	// coordinate the lockfile. However, we provide thread-safety to keep
	// things simple.
	packageLock.Lock()
	defer packageLock.Unlock()

	if r.closed {
		return nil, errors.New("cannot access config, repo not open")
	}
	return r.config, nil
}

func (r *FSRepo) FileManager() *filestore.FileManager {
	return r.filemgr
}

func (r *FSRepo) BackupConfig(prefix string) (string, error) {
	temp, err := ioutil.TempFile(r.path, "config-"+prefix)
	if err != nil {
		return "", err
	}
	defer temp.Close()

	configFilename, err := config.Filename(r.path)
	if err != nil {
		return "", err
	}

	orig, err := os.OpenFile(configFilename, os.O_RDONLY, 0600)
	if err != nil {
		return "", err
	}
	defer orig.Close()

	_, err = io.Copy(temp, orig)
	if err != nil {
		return "", err
	}

	return orig.Name(), nil
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
	// Do not use `*r.config = ...`. This will modify the *shared* config
	// returned by `r.Config`.
	r.config = updated
	return nil
}

// SetConfig updates the FSRepo's config. The user must not modify the config
// object after calling this method.
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

	if r.closed {
		return nil, errors.New("repo is closed")
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

	if r.closed {
		return errors.New("repo is closed")
	}

	filename, err := config.Filename(r.path)
	if err != nil {
		return err
	}
	// Load into a map so we don't end up writing any additional defaults to the config file.
	var mapconf map[string]interface{}
	if err := serialize.ReadConfigFile(filename, &mapconf); err != nil {
		return err
	}

	// Load private key to guard against it being overwritten.
	// NOTE: this is a temporary measure to secure this field until we move
	// keys out of the config file.
	pkval, err := common.MapGetKV(mapconf, config.PrivKeySelector)
	if err != nil {
		return err
	}

	// Set the key in the map.
	if err := common.MapSetKV(mapconf, key, value); err != nil {
		return err
	}

	// replace private key, in case it was overwritten.
	if err := common.MapSetKV(mapconf, config.PrivKeySelector, pkval); err != nil {
		return err
	}

	// This step doubles as to validate the map against the struct
	// before serialization
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
func (r *FSRepo) Datastore() repo.Datastore {
	packageLock.Lock()
	d := r.ds
	packageLock.Unlock()
	return d
}

// GetStorageUsage computes the storage space taken by the repo in bytes
func (r *FSRepo) GetStorageUsage() (uint64, error) {
	return ds.DiskUsage(r.Datastore())
}

func (r *FSRepo) SwarmKey() ([]byte, error) {
	repoPath := filepath.Clean(r.path)
	spath := filepath.Join(repoPath, swarmKeyFile)

	f, err := os.Open(spath)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
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
func isInitializedUnsynced(repoPath string) bool {
	return configIsInitialized(repoPath)
}
