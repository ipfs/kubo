package handshake

import (
	"errors"
	"fmt"

	updates "github.com/jbenet/go-ipfs/updates"

	semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
)

// currentVersion holds the current protocol version for a client running this code
var currentVersion *semver.Version

func init() {
	var err error
	currentVersion, err = semver.NewVersion("0.0.1")
	if err != nil {
		panic(fmt.Errorf("invalid protocol version: %v", err))
	}
}

// CurrentHandshake returns the current protocol version as a protobuf message
func CurrentHandshake() *Handshake1 {
	return NewHandshake1(currentVersion.String(), "go-ipfs/"+updates.Version)
}

// ErrVersionMismatch is returned when two clients don't share a protocol version
var ErrVersionMismatch = errors.New("protocol missmatch")

// Compatible checks wether two versions are compatible
// returns nil if they are fine
func Compatible(handshakeA, handshakeB *Handshake1) error {
	a, err := semver.NewVersion(*handshakeA.ProtocolVersion)
	if err != nil {
		return err
	}
	b, err := semver.NewVersion(*handshakeB.ProtocolVersion)
	if err != nil {
		return err
	}

	if a.Major != b.Major {
		return ErrVersionMismatch
	}

	return nil
}

// NewHandshake1 creates a new Handshake1 from the two strings
func NewHandshake1(protoVer, agentVer string) *Handshake1 {
	return &Handshake1{
		ProtocolVersion: &protoVer,
		AgentVersion:    &agentVer,
	}
}
