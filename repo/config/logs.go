package config

// LogsDefaultDirectory is the directory to store all IPFS event logs.
var LogsDefaultDirectory = "logs"

// Logs tracks the configuration of the event logger
type Logs struct {
	Filename   string
	MaxSizeMB  uint64
	MaxBackups uint64
	MaxAgeDays uint64
}

// LogsPath returns the default path for event logs given a configuration root
// (set an empty string to have the default configuration root)
func LogsPath(configroot string) (string, error) {
	return Path(configroot, LogsDefaultDirectory)
}
