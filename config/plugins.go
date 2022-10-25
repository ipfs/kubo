package config

type Plugins struct {
	Plugins map[string]Plugin
	// TODO: Loader Path? Leaving that out for now due to security concerns.
}

type Plugin struct {
	Disabled bool
	Config   interface{}
}
