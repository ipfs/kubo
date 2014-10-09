package config

import "time"

// Version regulates checking if the most recent version is run
type Version struct {
	Check             string        // "ignore" for do not check, "warn" and "error" for reacting when obsolete
	Current           string        // ipfs version for which config was generated
	UpdateCheckedTime time.Time     // timestamp for the last time API endpoint was checked for updates
	UpdateCheckPeriod time.Duration // time duration over which the update check will not be performed
}

// supported Version.Check values
const (
	CheckError  = "error"  // value for Version.Check to raise error and exit if version is obsolete
	CheckWarn   = "warn"   // value for Version.Check to show warning message if version is obsolete
	CheckIgnore = "ignore" // value for Version.Check to not perform update check
)

var defaultUpdateCheckPeriod = time.Hour * 48

// EligibleForUpdateCheck returns if update check API endpoint is needed for this specific runtime
func (v *Version) EligibleForUpdateCheck() bool {
	if v.Check == CheckIgnore || v.UpdateCheckedTime.Add(v.UpdateCheckPeriod).After(time.Now()) {
		return false
	}
	return true
}

// RecordCurrentUpdateCheck is called to record that update check was performed and showed that the running version is the most recent one
func (cfg *Config) RecordCurrentUpdateCheck(filename string) {
	cfg.Version.UpdateCheckedTime = time.Now()
	if cfg.Version.UpdateCheckPeriod == time.Duration(0) {
		// UpdateCheckPeriod was not initialized for some reason (e.g. config file used is broken)
		cfg.Version.UpdateCheckPeriod = defaultUpdateCheckPeriod
	}

	WriteConfigFile(filename, cfg)
}
