package fsrepo

import (
	common "github.com/jbenet/go-ipfs/repo/common"
	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
)

var _ component = &configComponent{}

// configComponent abstracts the config component of the FSRepo.
// NB: create with makeConfigComponent function.
type configComponent struct {
	path   string         // required at instantiation
	config *config.Config // assigned on Open()
}

// makeConfigComponent instantiates a valid configComponent.
func makeConfigComponent(path string) configComponent {
	return configComponent{path: path}
}

// fsrepoConfigInit initializes the FSRepo's configComponent.
func initConfigComponent(path string, conf *config.Config) error {
	if configComponentIsInitialized(path) {
		return nil
	}
	configFilename, err := config.Filename(path)
	if err != nil {
		return err
	}
	// initialization is the one time when it's okay to write to the config
	// without reading the config from disk and merging any user-provided keys
	// that may exist.
	if err := writeConfigFile(configFilename, conf); err != nil {
		return err
	}
	return nil
}

// Open returns an error if the config file is not present.
func (c *configComponent) Open() error {
	configFilename, err := config.Filename(c.path)
	if err != nil {
		return err
	}
	conf, err := load(configFilename)
	if err != nil {
		return err
	}
	c.config = conf
	return nil
}

// Close satisfies the fsrepoComponent interface.
func (c *configComponent) Close() error {
	return nil // config doesn't need to be closed.
}

func (c *configComponent) Config() *config.Config {
	return c.config
}

// SetConfig updates the config file.
func (c *configComponent) SetConfig(updated *config.Config) error {
	return c.setConfigUnsynced(updated)
}

// GetConfigKey retrieves only the value of a particular key.
func (c *configComponent) GetConfigKey(key string) (interface{}, error) {
	filename, err := config.Filename(c.path)
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
func (c *configComponent) SetConfigKey(key string, value interface{}) error {
	filename, err := config.Filename(c.path)
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
	// in order to get the updated values, read updated config from the
	// file-system.
	conf, err := config.FromMap(mapconf)
	if err != nil {
		return err
	}
	return c.setConfigUnsynced(conf) // TODO roll this into this method
}

// configComponentIsInitialized returns true if the repo is initialized at
// provided |path|.
func configComponentIsInitialized(path string) bool {
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
func (r *configComponent) setConfigUnsynced(updated *config.Config) error {
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
