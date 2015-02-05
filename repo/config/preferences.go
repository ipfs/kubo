package config

// Preferences stores local appearance and formatting settings
// for terminal and webUI output
type Preferences struct {
	// TerminalColors flag is for terminal text coloring
	// with escape characters
	TerminalColors bool
}

func PreferencesDefaultValue() Preferences {
	return Preferences{
		TerminalColors: false,
	}
}
