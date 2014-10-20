package config

import (
	"strconv"
	"time"
)

// Version regulates checking if the most recent version is run
type Version struct {
	// Current is the ipfs version for which config was generated
	Current string

	// Check signals how to react on updates:
	// - "ignore" for not checking
	// - "warn" for issuing a warning and proceeding
	// - "error" for exiting with an error
	Check string

	// CheckDate is a timestamp for the last time API endpoint was checked for updates
	CheckDate time.Time

	// CheckPeriod is the time duration over which the update check will not be performed
	// (Note: cannot use time.Duration because marshalling with json breaks it)
	CheckPeriod string

	// AutoUpdate is optional and has these these options:
	// - "never" do not auto-update
	// - "patch" auto-update on new patch versions
	// - "minor" auto-update on new minor (or patch) versions (Default)
	// - "major" auto-update on any new version
	AutoUpdate string
}

// supported Version.Check values
const (
	// CheckError value for Version.Check to raise error and exit if version is obsolete
	CheckError = "error"

	// CheckWarn value for Version.Check to show warning message if version is obsolete
	CheckWarn = "warn"

	// CheckIgnore value for Version.Check to not perform update check
	CheckIgnore = "ignore"
)

// supported Version.AutoUpdate values
// BUG(cryptix): make this a custom type that implements json.Unmarshaller() to verify values
const (
	UpdateNever = "never"
	UpdatePatch = "patch"
	UpdateMinor = "minor"
	UpdateMajor = "major"
)

// defaultCheckPeriod governs h
var defaultCheckPeriod = time.Hour * 48

func (v *Version) checkPeriodDuration() time.Duration {
	d, err := strconv.Atoi(v.CheckPeriod)
	if err != nil {
		log.Error("config.Version.CheckPeriod parse error. Using default.")
		return defaultCheckPeriod
	}
	return time.Duration(d)
}

// ShouldCheckForUpdate returns if update check API endpoint is needed for this specific runtime
func (v *Version) ShouldCheckForUpdate() bool {

	period := v.checkPeriodDuration()
	if v.Check == CheckIgnore || v.CheckDate.Add(period).After(time.Now()) {
		return false
	}
	return true
}

// RecordUpdateCheck is called to record that an update check was performed,
// showing that the running version is the most recent one.
func RecordUpdateCheck(cfg *Config, filename string) {
	cfg.Version.CheckDate = time.Now()

	if cfg.Version.CheckPeriod == "" {
		// CheckPeriod was not initialized for some reason (e.g. config file broken)
		cfg.Version.CheckPeriod = strconv.Itoa(int(defaultCheckPeriod))
	}

	WriteConfigFile(filename, cfg)
}
