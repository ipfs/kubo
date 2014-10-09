package updates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/coreos/go-semver/semver"
	u "github.com/jbenet/go-ipfs/util"
)

const (
	Version                   = "0.1.0" // actual current application's version literal
	EndpointURLLatestReleases = "https://api.github.com/repos/jbenet/go-ipfs/tags"
	VersionErrorShort         = `Warning: You are running version %s of go-ipfs. The latest version is %s.`
	VersionErrorLong          = `
  Warning: You are running version %s of go-ipfs. The latest version is %s.
  Since this is alpha software, it is strongly recommended you update.

  You can update go-ipfs by running

      ipfs version update

  You can silence this message by running

      ipfs config update.check ignore

  `
)

var currentVersion *semver.Version

func init() {
	var err error
	currentVersion, err = semver.NewVersion(Version)
	if err != nil {
		u.PErr("The const Version literal in version.go needs to be in semver format: %s \n", Version)
		os.Exit(1)
	}
}

func CheckForUpdates() error {
	resp, err := http.Get(EndpointURLLatestReleases)
	if err != nil {
		// can't reach the endpoint, coud be firewall, or no internet connection or something else
		// will just silently move on
		return nil
	}
	var body interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	releases, ok := body.([]interface{})
	if !ok {
		// the response body does not seem to meet specified Github API format
		// https://developer.github.com/v3/repos/#list-tags
		// will just silently move on
		return nil
	}
	for _, r := range releases {
		release, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		tagName, ok := release["name"].(string)
		if !ok {
			continue
		}
		if len(tagName) > 0 && tagName[0] == 'v' {
			// both 'v0.1.0' and '0.1.0' semver tagname conventions can be encountered
			tagName = tagName[1:]
		}
		releaseVersion, err := semver.NewVersion(tagName)
		if err != nil {
			continue
		}
		if currentVersion.LessThan(*releaseVersion) {
			return fmt.Errorf(VersionErrorLong, Version, tagName)
		}
	}
	return nil
}
