package config

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// CurrentVersionNumber is the current application's version literal
const CurrentVersionNumber = "0.3.4"

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
type AutoUpdateSetting int

// AutoUpdateSetting values
const (
	AutoUpdateNever AutoUpdateSetting = iota // do not auto-update
	AutoUpdatePatch                          // only on new patch versions
	AutoUpdateMinor                          // on new minor or patch versions (Default)
	AutoUpdateMajor                          // on all, even Major, version changes
)

// ErrUnknownAutoUpdateSetting is returned when an unknown value is read from the config
var ErrUnknownAutoUpdateSetting = errors.New("unknown value for AutoUpdate")

// defaultCheckPeriod governs h
var defaultCheckPeriod = time.Hour * 48

// UnmarshalJSON checks the input against known strings
func (s *AutoUpdateSetting) UnmarshalJSON(in []byte) error {

	switch strings.ToLower(string(in)) {
	case `"never"`:
		*s = AutoUpdateNever
	case `"major"`:
		*s = AutoUpdateMajor
	case `"minor"`:
		*s = AutoUpdateMinor
	case `"patch"`:
		*s = AutoUpdatePatch
	default:
		*s = AutoUpdateMinor
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
	case AutoUpdateNever:
		return "never"
	case AutoUpdateMajor:
		return "major"
	case AutoUpdateMinor:
		return "minor"
	case AutoUpdatePatch:
		return "patch"
	default:
		return ErrUnknownAutoUpdateSetting.Error()
	}
}

func (v *Version) checkPeriodDuration() time.Duration {
	d, err := strconv.Atoi(v.CheckPeriod)
	if err != nil {
		log.Warning("config.Version.CheckPeriod parse error. Using default.")
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

// VersionDefaultValue returns the default version config value (for init).
func VersionDefaultValue() Version {
	return Version{
		Current:     CurrentVersionNumber,
		Check:       "error",
		CheckPeriod: strconv.Itoa(int(defaultCheckPeriod)),
		AutoUpdate:  AutoUpdateMinor,
	}
}
