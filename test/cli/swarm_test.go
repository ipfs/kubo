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
		res := node.RunIPFS("swarm", "peers", "--enc=json", "--identify")
		var output expectedOutputType
		err := json.Unmarshal(res.Stdout.Bytes(), &output)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(output.Peers))
	})
	t.Run("ipfs swarm peers with flag identify outputs expected identify information about connected peers", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		otherNode := harness.NewT(t).NewNode().Init().StartDaemon()
		node.Connect(otherNode)

		res := node.RunIPFS("swarm", "peers", "--enc=json", "--identify")
		var output expectedOutputType
		err := json.Unmarshal(res.Stdout.Bytes(), &output)
		assert.NoError(t, err)
		actualID := output.Peers[0].Identify.ID
		actualPublicKey := output.Peers[0].Identify.PublicKey
		actualAgentVersion := output.Peers[0].Identify.AgentVersion
		actualAdresses := output.Peers[0].Identify.Addresses
		actualProtocols := output.Peers[0].Identify.Protocols

		expectedID := otherNode.PeerID().String()
		expectedAddresses := []string{fmt.Sprintf("%s/p2p/%s", otherNode.SwarmAddrs()[0], actualID)}

		assert.Equal(t, actualID, expectedID)
		assert.NotNil(t, actualPublicKey)
		assert.NotNil(t, actualAgentVersion)
		assert.Len(t, actualAdresses, 1)
		assert.Equal(t, expectedAddresses[0], actualAdresses[0])
		assert.Greater(t, len(actualProtocols), 0)
	})

	t.Run("ipfs swarm peers with flag identify outputs Identify field with data that matches calling ipfs id on a peer", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		otherNode := harness.NewT(t).NewNode().Init().StartDaemon()
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
}
