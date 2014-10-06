package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	semver "github.com/coreos/go-semver/semver"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	u "github.com/jbenet/go-ipfs/util"
)

// The IPFS version.
const (
	Version                   = "0.1.0"
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

var cmdIpfsVersion = &commander.Command{
	UsageLine: "version",
	Short:     "Show ipfs version information.",
	Long: `ipfs version - Show ipfs version information.

    Returns the current version of ipfs and exits.
  `,
	Run: versionCmd,
}

var currentVersion *semver.Version

func init() {
	var err error
	currentVersion, err = semver.NewVersion(Version)
	if err != nil {
		fmt.Printf("The const Version literal in version.go needs to be in semver format: %s \n", Version)
		os.Exit(1)
	}
	cmdIpfsVersion.Flag.Bool("number", false, "show only the number")
}

func versionCmd(c *commander.Command, _ []string) error {
	number := c.Flag.Lookup("number").Value.Get().(bool)
	if !number {
		u.POut("ipfs version ")
	}
	u.POut("%s\n", Version)
	return nil
}

func checkForUpdates() error {
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
