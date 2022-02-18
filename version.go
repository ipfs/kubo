package ipfs

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile
var CurrentCommit string

// CurrentVersionNumber is the current application's version literal
const CurrentVersionNumber = "0.12.0"

const ApiVersion = "/go-ipfs/" + CurrentVersionNumber + "/"

// GetUserAgentVersion is the libp2p user agent used by go-ipfs.
//
// Note: This will end in `/` when no commit is available. This is expected.
func GetUserAgentVersion() string {
	userAgent := "go-ipfs/" + CurrentVersionNumber + "/" + CurrentCommit
	if userAgentSuffix != "" {
		if CurrentCommit != "" {
			userAgent += "/"
		}
		userAgent += userAgentSuffix
	}
	return userAgent
}

var userAgentSuffix string

func SetUserAgentSuffix(suffix string) {
	userAgentSuffix = suffix
}
