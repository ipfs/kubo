package corefuse

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"

	fuseversion "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-fuse-version"
)

func init() {
	// this is a hack, but until we need to do it another way, this works.
	platformFuseChecks = darwinFuseCheckVersion
}

func darwinFuseCheckVersion() error {
	// on OSX, check FUSE version.
	if runtime.GOOS != "darwin" {
		return nil
	}

	ov, err := tryGFV()
	if err != nil {
		log.Debug(err)
		ov, err = trySysctl()
		if err != nil {
			log.Debug(err)
			return fmt.Errorf("cannot determine osxfuse version. is it installed?")
		}
	}

	log.Debug("mount: osxfuse version:", ov)
	if strings.HasPrefix(ov, "2.7.") || strings.HasPrefix(ov, "2.8.") {
		return nil
	}

	return fmt.Errorf("osxfuse version %s not supported.\n%s\n%s", ov,
		"Older versions of osxfuse have kernel panic bugs; please upgrade!",
		"https://github.com/jbenet/go-ipfs/issues/177")
}

func tryGFV() (string, error) {
	sys, err := fuseversion.LocalFuseSystems()
	if err != nil {
		log.Debug("mount: fuseversion:", "failed")
		return "", err
	}

	for _, s := range *sys {
		v := s.AgentVersion
		log.Debug("mount: fuseversion:", v)
		return v, nil
	}

	return "", fmt.Errorf("fuseversion: no system found")
}

func trySysctl() (string, error) {
	v, err := syscall.Sysctl("osxfuse.version.number")
	if err != nil {
		log.Debug("mount: sysctl osxfuse.version.number:", "failed")
		return "", err
	}
	log.Debug("mount: sysctl osxfuse.version.number:", v)
	return v, nil
}
