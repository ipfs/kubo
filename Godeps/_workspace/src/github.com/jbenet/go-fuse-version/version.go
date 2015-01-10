// package fuseversion simply exposes the version of FUSE installed
// in the user's machine. For reasoning, see:
// - https://github.com/jbenet/go-ipfs/issues/177
// - https://github.com/jbenet/go-ipfs/issues/202
// - https://github.com/osxfuse/osxfuse/issues/175#issuecomment-61888505
package fuseversion

type Systems map[string]FuseSystem

type FuseSystem struct {
	// FuseVersion is the version of the FUSE protocol
	FuseVersion string

	// AgentName identifies the system implementing FUSE, or Agent
	AgentName string

	// AgentVersion is the version of the Agent program
	// (it fights for the user! Sometimes it fights the user...)
	AgentVersion string
}

// LocalFuseSystems returns a map of FuseSystems, keyed by name.
// For example:
//
//  systems := fuseversion.LocalFuseSystems()
//  for n, sys := range systems {
//    fmt.Printf("%s, %s, %s", n, sys.FuseVersion, sys.AgentVersion)
//  }
//  // Outputs:
//  // OSXFUSE, , 2.7.2
//
func LocalFuseSystems() (*Systems, error) {
	return getLocalFuseSystems() // implemented by each platform
}

var notImplYet = `Error: not implemented for %s yet. :(
Please do it: https://github.com/jbenet/go-fuse-version`
