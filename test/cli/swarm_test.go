package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"

	"github.com/stretchr/testify/assert"
)



func TestSwarm(t *testing.T) {
	type identifyType struct {
		ID string
	}
	type peer struct {
	   Identify identifyType
	}
	type expectedOutputType struct {
		Peers []peer
	}

	t.Parallel()

	t.Run("ipfs swarm peers returns empty peers when a node is not connected to any peers", func (t *testing.T){
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		res := node.RunIPFS("swarm","peers","--enc=json","--identify")
		var output expectedOutputType
		err := json.Unmarshal(res.Stdout.Bytes(),&output)
		assert.Nil(t,err)
		assert.Equal(t,0,len(output.Peers))

	})
	t.Run("ipfs swarm peers with flag identify outputs expected results", func(t *testing.T) {
		t.Parallel()
		peerings := []harness.Peering{{From: 0, To: 1}}
		_, nodes := harness.CreatePeerNodes(t, 2, peerings)
		nodes.StartDaemons()
		assert.Eventuallyf(t,func () bool {
			res := nodes[1].RunIPFS("swarm","peers","--enc=json","--identify")
			var output expectedOutputType
			err := json.Unmarshal(res.Stdout.Bytes(),&output)
			if len(output.Peers) == 0 || err != nil {
				return false
			}
			return assert.Equal(t,output.Peers[0].Identify.ID,nodes[0].PeerID().String(),nodes[0].PeerID().String())  
		},20*time.Second, 10*time.Millisecond, "not peered")
	
	})
}
