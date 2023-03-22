package cli

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"

	"github.com/stretchr/testify/assert"
)

func TestSwarm(t *testing.T) {
	type identifyType struct {
		ID              string
		PublicKey       string
		Addresses       []string
		AgentVersion    string
		ProtocolVersion string
		Protocols       []string
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
		assert.Nil(t, err)
		assert.Equal(t, 0, len(output.Peers))

	})
	t.Run("ipfs swarm peers with flag identify outputs expected identify information about connected peers", func(t *testing.T) {
		t.Parallel()
		peerings := []harness.Peering{{From: 0, To: 1}}
		_, nodes := harness.CreatePeerNodes(t, 2, peerings)

		nodes.StartDaemons()
		assert.Eventuallyf(t, func() bool {
			res := nodes[1].RunIPFS("swarm", "peers", "--enc=json", "--identify")
			var output expectedOutputType
			err := json.Unmarshal(res.Stdout.Bytes(), &output)
			if len(output.Peers) == 0 || err != nil {
				return false
			}
			actualId := output.Peers[0].Identify.ID
			actualPublicKey := output.Peers[0].Identify.PublicKey
			actualAgentVersion := output.Peers[0].Identify.AgentVersion
			actualAdresses := output.Peers[0].Identify.Addresses
			actualProtocolVersion := output.Peers[0].Identify.ProtocolVersion
			actualProtocols := output.Peers[0].Identify.Protocols

			expectedId := nodes[0].PeerID().String()
			expectedAddresses := []string{fmt.Sprintf("%s/p2p/%s", nodes[0].SwarmAddrs()[0], actualId)}

			isResultValid := assert.Equal(t, actualId, expectedId) &&
				assert.NotNil(t, actualPublicKey) &&
				assert.NotNil(t, actualAgentVersion) &&
				assert.NotNil(t, actualProtocolVersion) &&
				assert.Len(t, actualAdresses, 1) &&
				assert.Equal(t, expectedAddresses[0], actualAdresses[0]) &&
				assert.Greater(t, len(actualProtocols), 0)
			return isResultValid
		}, 20*time.Second, 10*time.Millisecond, "error obtaining peer info")

	})
}
