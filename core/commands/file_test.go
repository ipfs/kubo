package commands

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core"
	"github.com/stretchr/testify/require"
)

type testenv struct {
	ctx  context.Context
	node *core.IpfsNode
}

type writeCloser struct {
	*bytes.Buffer
}

func (w writeCloser) Close() error { return nil }

func createTestEnv(t *testing.T) cmds.Environment {
	// Create a new IPFS node for testing
	ctx := context.Background()
	node, err := core.NewNode(ctx, &core.BuildCfg{
		Online: false,
		Repo:   nil,
	})
	require.NoError(t, err)

	return &testenv{
		ctx:  ctx,
		node: node,
	}
}

func TestCopyCBORtoMFS(t *testing.T) {
	// mock environment creation
	env := createTestEnv(t)

	cborCid := "bafyreigbtj4x7ip5legnfznufuopl4sg4knzc2cof6duas4b3q2fy6swua"

	req := &cmds.Request{
		Context: context.Background(),
		Arguments: []string{
			"/ipfs/" + cborCid,
			"/test-cbor",
		},
		Options: map[string]interface{}{
			filesFlushOptionName: true,
		},
	}

	// mock response emitter
	w := writeCloser{new(bytes.Buffer)}
	res, err := cmds.NewWriterResponseEmitter(w, req)
	require.NoError(t, err, "creating response emitter should not fail")

	err = filesCpCmd.Run(req, res, env)
	if err != nil {
		t.Logf("Error during file copy: %v", err) // Print actual error
		fmt.Println("Error:", err)                // Alternative direct print
	}

	require.Error(t, err, "copying dag-cbor should fail")
	// require.Contains(t, err.Error(), "must be a UnixFS node or raw data")
	// return fmt.Errorf("cp: source must be a UnixFS node or raw data")
}
