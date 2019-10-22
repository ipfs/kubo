package ipfs

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile
var CurrentCommit string

// CurrentVersionNumber is the current application's version literal
const CurrentVersionNumber = "0.5.0-dev"

const ApiVersion = "/go-ipfs/" + CurrentVersionNumber + "/"

// UserAgent is the libp2p user agent used by go-ipfs.
var UserAgent = ApiVersion + CurrentCommit
