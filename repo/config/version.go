package config

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile
var CurrentCommit string

// SystemPluginPath is the a system-global plugin path, this is set as a ldflag in the Makefile
var SystemPluginPath string

// CurrentVersionNumber is the current application's version literal
const CurrentVersionNumber = "0.4.14-dev"

const ApiVersion = "/go-ipfs/" + CurrentVersionNumber + "/"
