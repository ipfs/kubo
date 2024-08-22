package loader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	config "github.com/ipfs/kubo/config"
	"github.com/ipld/go-ipld-prime/multicodec"

	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	plugin "github.com/ipfs/kubo/plugin"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"

	logging "github.com/ipfs/go-log"
	opentracing "github.com/opentracing/opentracing-go"
)

var preloadPlugins []plugin.Plugin

// Preload adds one or more plugins to the preload list. This should _only_ be called during init.
func Preload(plugins ...plugin.Plugin) {
	preloadPlugins = append(preloadPlugins, plugins...)
}

var log = logging.Logger("plugin/loader")

var loadPluginFunc = func(string) ([]plugin.Plugin, error) {
	return nil, fmt.Errorf("unsupported platform %s", runtime.GOOS)
}

type loaderState int

const (
	loaderLoading loaderState = iota
	loaderInitializing
	loaderInitialized
	loaderInjecting
	loaderInjected
	loaderStarting
	loaderStarted
	loaderClosing
	loaderClosed
	loaderFailed
)

func (ls loaderState) String() string {
	switch ls {
	case loaderLoading:
		return "Loading"
	case loaderInitializing:
		return "Initializing"
	case loaderInitialized:
		return "Initialized"
	case loaderInjecting:
		return "Injecting"
	case loaderInjected:
		return "Injected"
	case loaderStarting:
		return "Starting"
	case loaderStarted:
		return "Started"
	case loaderClosing:
		return "Closing"
	case loaderClosed:
		return "Closed"
	case loaderFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// PluginLoader keeps track of loaded plugins.
//
// To use:
//  1. Load any desired plugins with Load and LoadDirectory. Preloaded plugins
//     will automatically be loaded.
//  2. Call Initialize to run all initialization logic.
//  3. Call Inject to register the plugins.
//  4. Optionally call Start to start plugins.
//  5. Call Close to close all plugins.
type PluginLoader struct {
	state   loaderState
	plugins []plugin.Plugin
	started []plugin.Plugin
	config  config.Plugins
	repo    string
}

// NewPluginLoader creates new plugin loader.
func NewPluginLoader(repo string) (*PluginLoader, error) {
	loader := &PluginLoader{plugins: make([]plugin.Plugin, 0, len(preloadPlugins)), repo: repo}
	if repo != "" {
		switch plugins, err := readPluginsConfig(repo, config.DefaultConfigFile); {
		case err == nil:
			loader.config = plugins
		case os.IsNotExist(err):
		default:
			return nil, err
		}
	}

	for _, v := range preloadPlugins {
		if err := loader.Load(v); err != nil {
			return nil, err
		}
	}

	if err := loader.LoadDirectory(filepath.Join(repo, "plugins")); err != nil {
		return nil, err
	}
	return loader, nil
}

// readPluginsConfig reads the Plugins section of the IPFS config, avoiding
// reading anything other than the Plugin section. That way, we're free to
// make arbitrary changes to all _other_ sections in migrations.
func readPluginsConfig(repoRoot string, userConfigFile string) (config.Plugins, error) {
	var cfg struct {
		Plugins config.Plugins
	}

	cfgPath, err := config.Filename(repoRoot, userConfigFile)
	if err != nil {
		return config.Plugins{}, err
	}

	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		return config.Plugins{}, err
	}
	defer cfgFile.Close()

	err = json.NewDecoder(cfgFile).Decode(&cfg)
	if err != nil {
		return config.Plugins{}, err
	}

	return cfg.Plugins, nil
}

func (loader *PluginLoader) assertState(state loaderState) error {
	if loader.state != state {
		return fmt.Errorf("loader state must be %s, was %s", state, loader.state)
	}
	return nil
}

func (loader *PluginLoader) transition(from, to loaderState) error {
	if err := loader.assertState(from); err != nil {
		return err
	}
	loader.state = to
	return nil
}

// Load loads a plugin into the plugin loader.
func (loader *PluginLoader) Load(pl plugin.Plugin) error {
	if err := loader.assertState(loaderLoading); err != nil {
		return err
	}

	name := pl.Name()

	for _, p := range loader.plugins {
		if p.Name() == name {
			// plugin is already loaded
			return fmt.Errorf(
				"plugin: %s, is duplicated in version: %s, "+
					"while trying to load dynamically: %s",
				name, p.Version(), pl.Version())
		}
	}

	if loader.config.Plugins[name].Disabled {
		log.Infof("not loading disabled plugin %s", name)
		return nil
	}
	loader.plugins = append(loader.plugins, pl)
	return nil
}

// LoadDirectory loads a directory of plugins into the plugin loader.
func (loader *PluginLoader) LoadDirectory(pluginDir string) error {
	if err := loader.assertState(loaderLoading); err != nil {
		return err
	}
	newPls, err := loadDynamicPlugins(pluginDir)
	if err != nil {
		return err
	}

	for _, pl := range newPls {
		if err := loader.Load(pl); err != nil {
			return err
		}
	}
	return nil
}

func loadDynamicPlugins(pluginDir string) ([]plugin.Plugin, error) {
	_, err := os.Stat(pluginDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var plugins []plugin.Plugin

	err = filepath.Walk(pluginDir, func(fi string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if fi != pluginDir {
				log.Warnf("found directory inside plugins directory: %s", fi)
			}
			return nil
		}

		if info.Mode().Perm()&0o111 == 0 {
			// file is not executable let's not load it
			// this is to prevent loading plugins from for example non-executable
			// mounts, some /tmp mounts are marked as such for security
			log.Errorf("non-executable file in plugins directory: %s", fi)
			return nil
		}

		if newPlugins, err := loadPluginFunc(fi); err == nil {
			plugins = append(plugins, newPlugins...)
		} else {
			return fmt.Errorf("loading plugin %s: %s", fi, err)
		}
		return nil
	})

	return plugins, err
}

// Initialize initializes all loaded plugins.
func (loader *PluginLoader) Initialize() error {
	if err := loader.transition(loaderLoading, loaderInitializing); err != nil {
		return err
	}
	for _, p := range loader.plugins {
		err := p.Init(&plugin.Environment{
			Repo:   loader.repo,
			Config: loader.config.Plugins[p.Name()].Config,
		})
		if err != nil {
			loader.state = loaderFailed
			return err
		}
	}

	return loader.transition(loaderInitializing, loaderInitialized)
}

// Inject hooks all the plugins into the appropriate subsystems.
func (loader *PluginLoader) Inject() error {
	if err := loader.transition(loaderInitialized, loaderInjecting); err != nil {
		return err
	}

	for _, pl := range loader.plugins {
		if pl, ok := pl.(plugin.PluginIPLD); ok {
			err := injectIPLDPlugin(pl)
			if err != nil {
				loader.state = loaderFailed
				return err
			}
		}
		if pl, ok := pl.(plugin.PluginTracer); ok {
			err := injectTracerPlugin(pl)
			if err != nil {
				loader.state = loaderFailed
				return err
			}
		}
		if pl, ok := pl.(plugin.PluginDatastore); ok {
			err := injectDatastorePlugin(pl)
			if err != nil {
				loader.state = loaderFailed
				return err
			}
		}
		if pl, ok := pl.(plugin.PluginFx); ok {
			err := injectFxPlugin(pl)
			if err != nil {
				loader.state = loaderFailed
				return err
			}
		}
	}

	return loader.transition(loaderInjecting, loaderInjected)
}

// Start starts all long-running plugins.
func (loader *PluginLoader) Start(node *core.IpfsNode) error {
	if err := loader.transition(loaderInjected, loaderStarting); err != nil {
		return err
	}
	iface, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}
	for _, pl := range loader.plugins {
		if pl, ok := pl.(plugin.PluginDaemon); ok {
			err := pl.Start(iface)
			if err != nil {
				_ = loader.Close()
				return err
			}
			loader.started = append(loader.started, pl)
		}
		if pl, ok := pl.(plugin.PluginDaemonInternal); ok {
			err := pl.Start(node)
			if err != nil {
				_ = loader.Close()
				return err
			}
			loader.started = append(loader.started, pl)
		}
	}

	return loader.transition(loaderStarting, loaderStarted)
}

// Close stops all long-running plugins.
func (loader *PluginLoader) Close() error {
	switch loader.state {
	case loaderClosing, loaderFailed, loaderClosed:
		// nothing to do.
		return nil
	}
	loader.state = loaderClosing

	var errs []string
	started := loader.started
	loader.started = nil
	for _, pl := range started {
		if closer, ok := pl.(io.Closer); ok {
			err := closer.Close()
			if err != nil {
				errs = append(errs, fmt.Sprintf(
					"error closing plugin %s: %s",
					pl.Name(),
					err.Error(),
				))
			}
		}
	}
	if errs != nil {
		loader.state = loaderFailed
		return errors.New(strings.Join(errs, "\n"))
	}
	loader.state = loaderClosed
	return nil
}

func injectDatastorePlugin(pl plugin.PluginDatastore) error {
	return fsrepo.AddDatastoreConfigHandler(pl.DatastoreTypeName(), pl.DatastoreConfigParser())
}

func injectIPLDPlugin(pl plugin.PluginIPLD) error {
	return pl.Register(multicodec.DefaultRegistry)
}

func injectTracerPlugin(pl plugin.PluginTracer) error {
	log.Warn("Tracer plugins are deprecated, it's recommended to configure an OpenTelemetry collector instead.")
	tracer, err := pl.InitTracer()
	if err != nil {
		return err
	}
	opentracing.SetGlobalTracer(tracer)
	return nil
}

func injectFxPlugin(pl plugin.PluginFx) error {
	core.RegisterFXOptionFunc(pl.Options)
	return nil
}
