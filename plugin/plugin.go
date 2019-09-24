package plugin

// Environment is the environment passed into the plugin on init.
type Environment struct {
	// Path to the IPFS repo.
	Repo string

	// The plugin's config, if specified.
	Config interface{}
}

// Plugin is the base interface for all kinds of go-ipfs plugins
// It will be included in interfaces of different Plugins
//
// Optionally, Plugins can implement io.Closer if they want to
// have a termination step when unloading.
type Plugin interface {
	// Name should return unique name of the plugin
	Name() string

	// Version returns current version of the plugin
	Version() string

	// Init is called once when the Plugin is being loaded
	// The plugin is passed an environment containing the path to the
	// (possibly uninitialized) IPFS repo and the plugin's config.
	Init(env *Environment) error
}
