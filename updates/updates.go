package updates

import (
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/inconshreveable/go-update"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/inconshreveable/go-update/check"
	u "github.com/jbenet/go-ipfs/util"
)

const (
	Version           = "0.1.0" // actual current application's version literal
	UpdateEndpointURL = "https://api.equinox.io/1/Updates"
	UpdateAppID       = "ap_ywkPmAR40q4EfdikN9Jh2hgIHi"
)

var log = u.Logger("updates")

var currentVersion *semver.Version

func init() {
	var err error
	currentVersion, err = parseVersion()
	if err != nil {
		log.Error("illegal version number in code: %q\n", Version)
		os.Exit(1)
	}
}

func parseVersion() (*semver.Version, error) {
	return semver.NewVersion(Version)
}

func CheckForUpdate() (*check.Result, error) {
	param := check.Params{
		AppVersion: Version,
		AppId:      UpdateAppID,
		Channel:    "stable",
	}

	return param.CheckForUpdate(UpdateEndpointURL, update.New())
}
