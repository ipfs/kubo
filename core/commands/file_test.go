package commands

import (
	"context"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/stretchr/testify/require"
)

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
	res := new(cmds.EmptyResponse)

	// mock environment creation
	env := &cmdenv.Environment{}

	err := filesCpCmd.Run(req, res, env)

	require.Error(t, err, "copying dag-cbor should fail")
	require.Contains(t, err.Error(), "dag-cbor not supported", "must be a UnixFS node or raw data",
		"error should indicate invalid node type")
}
