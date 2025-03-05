package commands

import (
	"context"
	"io"
	"testing"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	cmds "github.com/ipfs/go-ipfs-cmds"
	coremock "github.com/ipfs/kubo/core/mock"
	"github.com/stretchr/testify/require"
)

func TestFilesCp_DagCborNodeFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdCtx, err := coremock.MockCmdsCtx()
	require.NoError(t, err)

	node, err := cmdCtx.ConstructNode()
	require.NoError(t, err)

	invalidData := []byte{0x00}
	protoNode := dag.NodeWithData(invalidData)
	err = node.DAG.Add(ctx, protoNode)
	require.NoError(t, err)

	req := &cmds.Request{
		Context: ctx,
		Arguments: []string{
			"/ipfs/" + protoNode.Cid().String(),
			"/test-destination",
		},
		Options: map[string]interface{}{
			"force": false,
		},
	}

	_, pw := io.Pipe()
	res, err := cmds.NewWriterResponseEmitter(pw, req)
	require.NoError(t, err)

	err = filesCpCmd.Run(req, res, &cmdCtx)
	require.Error(t, err)
	require.ErrorContains(t, err, "cp: source must be a valid UnixFS (dag-pb or raw codec)")
}
