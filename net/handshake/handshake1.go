package handshake

import (
	"errors"
	"fmt"

	pb "github.com/jbenet/go-ipfs/net/handshake/pb"
	updates "github.com/jbenet/go-ipfs/updates"

	semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
)

// ipfsVersion holds the current protocol version for a client running this code
var ipfsVersion *semver.Version
var clientVersion = "go-ipfs/" + updates.Version

func init() {
	var err error
	ipfsVersion, err = semver.NewVersion("0.0.1")
	if err != nil {
		panic(fmt.Errorf("invalid protocol version: %v", err))
	}
}

// Handshake1Msg returns the current protocol version as a protobuf message
func Handshake1Msg() *pb.Handshake1 {
	return NewHandshake1(ipfsVersion.String(), clientVersion)
}

// ErrVersionMismatch is returned when two clients don't share a protocol version
var ErrVersionMismatch = errors.New("protocol missmatch")

// Handshake1Compatible checks whether two versions are compatible
// returns nil if they are fine
func Handshake1Compatible(handshakeA, handshakeB *pb.Handshake1) error {
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
func NewHandshake1(protoVer, agentVer string) *pb.Handshake1 {
	return &pb.Handshake1{
		ProtocolVersion: &protoVer,
		AgentVersion:    &agentVer,
	}
}
