package handshake

import (
	"errors"
	"fmt"

	config "github.com/jbenet/go-ipfs/config"
	pb "github.com/jbenet/go-ipfs/p2p/net/handshake/pb"
	u "github.com/jbenet/go-ipfs/util"

	semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
)

var log = u.Logger("handshake")

// IpfsVersion holds the current protocol version for a client running this code
var IpfsVersion *semver.Version
var ClientVersion = "go-ipfs/" + config.CurrentVersionNumber

func init() {
	var err error
	IpfsVersion, err = semver.NewVersion("0.0.1")
	if err != nil {
		panic(fmt.Errorf("invalid protocol version: %v", err))
	}
}

// Handshake1Msg returns the current protocol version as a protobuf message
func Handshake1Msg() *pb.Handshake1 {
	return NewHandshake1(IpfsVersion.String(), ClientVersion)
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
	if protoVer == "" {
		protoVer = IpfsVersion.String()
	}
	if agentVer == "" {
		agentVer = ClientVersion
	}

	return &pb.Handshake1{
		ProtocolVersion: &protoVer,
		AgentVersion:    &agentVer,
	}
}
