package commands

import (
	"context"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core"
	"github.com/stretchr/testify/require"
)

type testenv struct {
	ctx  context.Context
	node *core.IpfsNode
}

func createTestEnvironment(t *testing.T) cmds.Environment {
	// Create a new IPFS node for testing
	node, err := core.NewNode(context.Background(), &core.BuildCfg{
		Online: false,
		Repo:   nil,
	})
	require.NoError(t, err)

	return &testenv{
		ctx:  context.Background(),
		node: node,
	}
}

func TestCopyCBORtoMFS(t *testing.T) {
	ctx := context.Background()

	cborCid := "bafyreigbtj4x7ip5legnfznufuopl4sg4knzc2cof6duas4b3q2fy6swua"

	req := &cmds.Request{
		Context: ctx,
		Arguments: []string{
			"/ipfs/" + cborCid,
			"/test-cbor",
		},
		Options: map[string]interface{}{
			filesFlushOptionName: true,
		},
	}

	// mock response emitter
	res, err := cmds.NewWriterResponseEmitter(nil, nil)
	require.NoError(t, err, "creating response emitter should not fail")

	// mock environment creation
	env := createTestEnvironment(t)

	err = filesCpCmd.Run(req, res, env)

	require.Error(t, err, "copying dag-cbor should fail")
	require.Contains(t, err.Error(), "dag-cbor not supported", "must be a UnixFS node or raw data",
		"error should indicate invalid node type")
}
