package core

import (
	"errors"
	"fmt"
	versioncmp "github.com/hashicorp/go-version"
	version "github.com/ipfs/kubo"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	pstore "github.com/libp2p/go-libp2p/core/peerstore"
	"strings"
	"time"
)

type VersionCheckOutput struct {
	IsOutdated bool
	Msg        string
}

func StartVersionChecker(nd *IpfsNode) {
	ticker := time.NewTicker(time.Hour)
	go func() {
		for {
			output, err := CheckVersion(nd, 0.1)
			if err != nil {
				log.Errorw("Failed to check version", "error", err)
			}
			if output.IsOutdated {
				fmt.Println(output.Msg)
			}

			select {
			case <-nd.Process.Closing():
				return
			case <-ticker.C:
				continue
			}
		}
	}()
}

func CheckVersion(nd *IpfsNode, newerFraction float64) (VersionCheckOutput, error) {
	ourVersion, err := versioncmp.NewVersion(strings.Replace(version.CurrentVersionNumber, "-dev", "", -1))
	if err != nil {
		return VersionCheckOutput{}, fmt.Errorf("could not parse our own version %s: %w",
			version.CurrentVersionNumber, err)
	}

	greatestVersionSeen := ourVersion
	totalPeersCounted := 1 // Us (and to avoid division-by-zero edge case).
	withGreaterVersion := 0

	recordPeerVersion := func(agentVersion string) {
		// We process the version as is it assembled in GetUserAgentVersion.
		segments := strings.Split(agentVersion, "/")
		if len(segments) < 2 {
			return
		}
		if segments[0] != "kubo" {
			return
		}
		versionNumber := segments[1] // As in our CurrentVersionNumber.

		// Ignore development releases.
		if strings.Contains(versionNumber, "-dev") {
			return
		}
		if strings.Contains(versionNumber, "-rc") {
			return
		}

		peerVersion, err := versioncmp.NewVersion(versionNumber)
		if err != nil {
			// Do not error on invalid remote versions, just ignore.
			return
		}

		// Valid peer version number.
		totalPeersCounted += 1
		if ourVersion.LessThan(peerVersion) {
			withGreaterVersion += 1
		}
		if peerVersion.GreaterThan(greatestVersionSeen) {
			greatestVersionSeen = peerVersion
		}
	}

	// Logic taken from `ipfs stats dht` command.
	if nd.DHTClient != nd.DHT {
		client, ok := nd.DHTClient.(*fullrt.FullRT)
		if !ok {
			return VersionCheckOutput{}, errors.New("could not generate stats for the WAN DHT client type")
		}
		for _, p := range client.Stat() {
			if ver, err := nd.Peerstore.Get(p, "AgentVersion"); err == nil {
				recordPeerVersion(ver.(string))
			} else if errors.Is(err, pstore.ErrNotFound) {
				// ignore
			} else {
				// this is a bug, usually.
				log.Errorw(
					"failed to get agent version from peerstore",
					"error", err,
				)
			}
		}
	} else {
		for _, pi := range nd.DHT.WAN.RoutingTable().GetPeerInfos() {
			if ver, err := nd.Peerstore.Get(pi.Id, "AgentVersion"); err == nil {
				recordPeerVersion(ver.(string))
			} else if errors.Is(err, pstore.ErrNotFound) {
				// ignore
			} else {
				// this is a bug, usually.
				log.Errorw(
					"failed to get agent version from peerstore",
					"error", err,
				)
			}
		}
	}

	if (float64(withGreaterVersion) / float64(totalPeersCounted)) > newerFraction {
		return VersionCheckOutput{
			IsOutdated: true,
			Msg:        fmt.Sprintf("⚠️WARNING: this Kubo node is running an outdated version compared to other peers, update to %s\n", greatestVersionSeen.String()),
		}, nil
	} else {
		return VersionCheckOutput{}, nil
	}
}
