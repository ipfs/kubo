package component

import (
	"strconv"

	common "github.com/jbenet/go-ipfs/repo/common"
	config "github.com/jbenet/go-ipfs/repo/config"
	serialize "github.com/jbenet/go-ipfs/repo/fsrepo/serialize"
	util "github.com/jbenet/go-ipfs/util"
)

var _ Component = &ConfigComponent{}
var _ Initializer = InitConfigComponent
var _ InitializationChecker = ConfigComponentIsInitialized

// ConfigComponent abstracts the config component of the FSRepo.
// NB: create with makeConfigComponent function.
// NOT THREAD-SAFE
type ConfigComponent struct {
	path   string         // required at instantiation
	config *config.Config // assigned on Open()
}

// fsrepoConfigInit initializes the FSRepo's ConfigComponent.
func InitConfigComponent(path string, conf *config.Config) error {
	if ConfigComponentIsInitialized(path) {
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

// Open returns an error if the config file is not present. This component is
// always called with a nil config parameter. Other components rely on the
// config, to keep the interface uniform, it is special-cased.
func (c *ConfigComponent) Open(_ *config.Config) error {
	configFilename, err := config.Filename(c.path)
	if err != nil {
		return err
	}
	conf, err := serialize.Load(configFilename)
	if err != nil {
		return err
	}
	c.config = conf
	return nil
}

// Close satisfies the fsrepoComponent interface.
func (c *ConfigComponent) Close() error {
	return nil // config doesn't need to be closed.
}

func (c *ConfigComponent) Config() *config.Config {
	return c.config
}

// SetConfig updates the config file.
func (c *ConfigComponent) SetConfig(updated *config.Config) error {
	return c.setConfigUnsynced(updated)
}

// GetConfigKey retrieves only the value of a particular key.
func (c *ConfigComponent) GetConfigKey(key string) (interface{}, error) {
	filename, err := config.Filename(c.path)
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
func (c *ConfigComponent) SetConfigKey(key string, value interface{}) error {
	filename, err := config.Filename(c.path)
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
	return c.setConfigUnsynced(conf) // TODO roll this into this method
}

func (c *ConfigComponent) SetPath(p string) {
	c.path = p
}

// ConfigComponentIsInitialized returns true if the repo is initialized at
// provided |path|.
func ConfigComponentIsInitialized(path string) bool {
	configFilename, err := config.Filename(path)
	if err != nil {
		return false
	}
	if !util.FileExists(configFilename) {
		return false
	}
	return true
}

// setConfigUnsynced is for private use.
func (r *ConfigComponent) setConfigUnsynced(updated *config.Config) error {
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
