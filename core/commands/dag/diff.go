package dagcmd

import (
	"encoding/json"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"io/ioutil"

	"github.com/wI2L/jsondiff"
)

func dagDiff(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	getArgNodeAsJson := func(argNumber int) ([]byte, error) {
		r, err := getNodeWithCodec(req.Context, req.Arguments[argNumber], "dag-json", api)
		if err != nil {
			return nil, err
		}

		jsonOutput, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}

		return jsonOutput, nil
	}

	nodeBefore, err := getArgNodeAsJson(0)
	if err != nil {
		return err
	}
	nodeAfter, err := getArgNodeAsJson(1)
	if err != nil {
		return err
	}

	patch, err := jsondiff.CompareJSONOpts(nodeBefore, nodeAfter, jsondiff.Invertible())
	if err != nil {
		return err
	}

	indented, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		return err
	}

	if err := res.Emit(DagDiff{string(indented)}); err != nil {
		return err
	}

	return nil
}
