package updates

import (
	"fmt"
	"os"

	"github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"

	semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	update "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/inconshreveable/go-update"
	check "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/inconshreveable/go-update/check"
)

const (
	// Version is the current application's version literal
	Version = "0.1.1"

	updateEndpointURL = "https://api.equinox.io/1/Updates"
	updateAppID       = "CHANGEME"

	updatePubKey = `-----BEGIN RSA PUBLIC KEY-----
CHANGEME
-----END RSA PUBLIC KEY-----`
)

var log = u.Logger("updates")

var currentVersion *semver.Version

func init() {
	var err error
	currentVersion, err = parseVersion()
	if err != nil {
		log.Error("invalid version number in code (must be semver): %q\n", Version)
		os.Exit(1)
	}
}

func parseVersion() (*semver.Version, error) {
	return semver.NewVersion(Version)
}

// CheckForUpdate checks the equinox.io api if there is an update available
func CheckForUpdate() (*check.Result, error) {
	param := check.Params{
		AppVersion: Version,
		AppId:      updateAppID,
		Channel:    "stable",
	}

	up, err := update.New().VerifySignatureWithPEM([]byte(updatePubKey))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse public key: %v", err)
	}

	return param.CheckForUpdate(updateEndpointURL, up)
}

// Apply cheks if the running process is able to update itself
// and than updates to the passed release
func Apply(rel *check.Result) error {
	if err := update.New().CanUpdate(); err != nil {
		return err
	}

	if err, errRecover := rel.Update(); err != nil {
		err = fmt.Errorf("Update failed: %v\n", err)
		if errRecover != nil {
			err = fmt.Errorf("%s\nRecovery failed! Cause: %v\nYou may need to recover manually", err, errRecover)
		}
		return err
	}

	return nil
}

// ShouldAutoUpdate decides wether a new version should be applied
// checks against config setting and new version string. returns false in case of error
func ShouldAutoUpdate(setting config.AutoUpdateSetting, newVer string) bool {
	if setting == config.UpdateNever {
		return false
	}

	nv, err := semver.NewVersion(newVer)
	if err != nil {
		log.Error("could not parse version string: %s", err)
		return false
	}

	n := nv.Slice()
	c := currentVersion.Slice()

	switch setting {

	case config.UpdatePatch:
		if n[0] < c[0] {
			return false
		}

		if n[1] < c[1] {
			return false
		}

		return n[2] > c[2]

	case config.UpdateMinor:
		if n[0] != c[0] {
			return false
		}

		return n[1] > c[1] || (n[1] == c[1] && n[2] > c[2])

	case config.UpdateMajor:
		for i := 0; i < 3; i++ {
			if n[i] < c[i] {
				return false
			}
		}
		return true
	}

	return false
}
