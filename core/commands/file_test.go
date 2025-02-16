package commands

import (
	"context"
	"io"
	"strings"
	"testing"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	cmds "github.com/ipfs/go-ipfs-cmds"
	coremock "github.com/ipfs/kubo/core/mock"
)

func TestFilesCp_DagCborNodeFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdCtx, err := coremock.MockCmdsCtx()
	if err != nil {
		t.Fatal(err)
	}

	node, err := cmdCtx.ConstructNode()
	if err != nil {
		t.Fatal(err)
	}

	cborData := []byte{0x80}
	cborNode := dag.NewRawNode(cborData)
	err = node.DAG.Add(ctx, cborNode)
	if err != nil {
		t.Fatal(err)
	}

	req := &cmds.Request{
		Context: ctx,
		Arguments: []string{
			"/ipfs/" + cborNode.Cid().String(),
			"/test-destination",
		},
		Options: map[string]interface{}{
			"force": false,
		},
	}

	_, pw := io.Pipe()
	res, err := cmds.NewWriterResponseEmitter(pw, req)
	if err != nil {
		t.Fatal(err)
	}

	err = filesCpCmd.Run(req, res, &cmdCtx)

	if err == nil {
		t.Fatal("expected error but got nil")
	}

	expectedErr := "cp: source must be a UnixFS node or raw data"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("got error %q, want %q", err.Error(), expectedErr)
	}
}
