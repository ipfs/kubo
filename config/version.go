package config

import (
	"errors"
	"strconv"
	"strings"
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

	// AutoUpdate is optional
	AutoUpdate AutoUpdateSetting
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

// AutoUpdateSetting implements json.Unmarshaler to check values in config
// supported values:
// 	"never" - do not auto-update
// 	"patch" - auto-update on new patch versions
// 	"minor" - auto-update on new minor (or patch) versions (Default)
// 	"major" - auto-update on any new version
type AutoUpdateSetting int

// UnmarshalJSON checks the input against known strings
func (s *AutoUpdateSetting) UnmarshalJSON(in []byte) error {

	switch strings.ToLower(string(in)) {
	case `"never"`:
		*s = UpdateNever
	case `"major"`:
		*s = UpdateMajor
	case `"minor"`:
		*s = UpdateMinor
	case `"patch"`:
		*s = UpdatePatch
	default:
		*s = UpdateMinor
		return ErrUnknownAutoUpdateSetting
	}
	return nil
}

// MarshalJSON converts the value back to JSON string
func (s AutoUpdateSetting) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// String converts valye to human readable string
func (s AutoUpdateSetting) String() string {
	switch s {
	case UpdateNever:
		return "never"
	case UpdateMajor:
		return "major"
	case UpdateMinor:
		return "minor"
	case UpdatePatch:
		return "patch"
	default:
		return ErrUnknownAutoUpdateSetting.Error()
	}
}

// ErrUnknownAutoUpdateSetting is returned when an unknown value is read from the config
var ErrUnknownAutoUpdateSetting = errors.New("unknown value for AutoUpdate")

const (
	UpdateMinor AutoUpdateSetting = iota // first value so that it is the zero value and thus the default
	UpdatePatch
	UpdateMajor
	UpdateNever
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
