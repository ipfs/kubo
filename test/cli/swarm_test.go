package cli

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"

	"github.com/stretchr/testify/assert"
)

// TODO: Migrate the rest of the sharness swarm test.
func TestSwarm(t *testing.T) {
	type identifyType struct {
		ID           string
		PublicKey    string
		Addresses    []string
		AgentVersion string
		Protocols    []string
	}
	type peer struct {
		Identify identifyType
	}
	type expectedOutputType struct {
		Peers []peer
	}

	t.Parallel()

	t.Run("ipfs swarm peers returns empty peers when a node is not connected to any peers", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()
		res := node.RunIPFS("swarm", "peers", "--enc=json", "--identify")
		var output expectedOutputType
		err := json.Unmarshal(res.Stdout.Bytes(), &output)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(output.Peers))
	})
	t.Run("ipfs swarm peers with flag identify outputs expected identify information about connected peers", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()
		otherNode := harness.NewT(t).NewNode().Init().StartDaemon()
		defer otherNode.StopDaemon()
		node.Connect(otherNode)

		res := node.RunIPFS("swarm", "peers", "--enc=json", "--identify")
		var output expectedOutputType
		err := json.Unmarshal(res.Stdout.Bytes(), &output)
		assert.NoError(t, err)
		actualID := output.Peers[0].Identify.ID
		actualPublicKey := output.Peers[0].Identify.PublicKey
		actualAgentVersion := output.Peers[0].Identify.AgentVersion
		actualAddresses := output.Peers[0].Identify.Addresses
		actualProtocols := output.Peers[0].Identify.Protocols

		expectedID := otherNode.PeerID().String()
		expectedAddresses := []string{fmt.Sprintf("%s/p2p/%s", otherNode.SwarmAddrs()[0], actualID)}

		assert.Equal(t, actualID, expectedID)
		assert.NotNil(t, actualPublicKey)
		assert.NotNil(t, actualAgentVersion)
		assert.Len(t, actualAddresses, 1)
		assert.Equal(t, expectedAddresses[0], actualAddresses[0])
		assert.Greater(t, len(actualProtocols), 0)
	})

	t.Run("ipfs swarm peers with flag identify outputs Identify field with data that matches calling ipfs id on a peer", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()
		otherNode := harness.NewT(t).NewNode().Init().StartDaemon()
		defer otherNode.StopDaemon()
		node.Connect(otherNode)

		otherNodeIDResponse := otherNode.RunIPFS("id", "--enc=json")
		var otherNodeIDOutput identifyType
		err := json.Unmarshal(otherNodeIDResponse.Stdout.Bytes(), &otherNodeIDOutput)
		assert.NoError(t, err)
		res := node.RunIPFS("swarm", "peers", "--enc=json", "--identify")

		var output expectedOutputType
		err = json.Unmarshal(res.Stdout.Bytes(), &output)
		assert.NoError(t, err)
		outputIdentify := output.Peers[0].Identify

		assert.Equal(t, outputIdentify.ID, otherNodeIDOutput.ID)
		assert.Equal(t, outputIdentify.PublicKey, otherNodeIDOutput.PublicKey)
		assert.Equal(t, outputIdentify.AgentVersion, otherNodeIDOutput.AgentVersion)
		assert.ElementsMatch(t, outputIdentify.Addresses, otherNodeIDOutput.Addresses)
		assert.ElementsMatch(t, outputIdentify.Protocols, otherNodeIDOutput.Protocols)
	})

	t.Run("ipfs swarm addrs autonat returns valid reachability status", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		res := node.RunIPFS("swarm", "addrs", "autonat", "--enc=json")
		assert.NoError(t, res.Err)

		var output struct {
			Reachability string   `json:"reachability"`
			Reachable    []string `json:"reachable"`
			Unreachable  []string `json:"unreachable"`
			Unknown      []string `json:"unknown"`
		}
		err := json.Unmarshal(res.Stdout.Bytes(), &output)
		assert.NoError(t, err)

		// Reachability must be one of the valid states
		// Note: network.Reachability constants use capital first letter
		validStates := []string{"Public", "Private", "Unknown"}
		assert.Contains(t, validStates, output.Reachability,
			"Reachability should be one of: Public, Private, Unknown")

		// For a newly started node, reachability is typically Unknown initially
		// as AutoNAT hasn't completed probing yet. This is expected behavior.
		// The important thing is that the command runs and returns valid data.
		totalAddrs := len(output.Reachable) + len(output.Unreachable) + len(output.Unknown)
		t.Logf("Reachability: %s, Total addresses: %d (reachable: %d, unreachable: %d, unknown: %d)",
			output.Reachability, totalAddrs, len(output.Reachable), len(output.Unreachable), len(output.Unknown))
	})
}
