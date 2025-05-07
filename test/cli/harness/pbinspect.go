package harness

import (
	"bytes"
	"encoding/json"
)

// InspectPBNode uses dag-json output of 'ipfs dag get' to inspect
// "Logical Format" of DAG-PB as defined in
// https://web.archive.org/web/20250403194752/https://ipld.io/specs/codecs/dag-pb/spec/#logical-format
// (mainly used for inspecting Links without depending on any libraries)
func (n *Node) InspectPBNode(cid string) (PBNode, error) {
	log.Debugf("node %d dag get %s as dag-json", n.ID, cid)

	var root PBNode
	var dagJsonOutput bytes.Buffer
	res := n.Runner.MustRun(RunRequest{
		Path:    n.IPFSBin,
		Args:    []string{"dag", "get", "--output-codec=dag-json", cid},
		CmdOpts: []CmdOpt{RunWithStdout(&dagJsonOutput)},
	})
	if res.Err != nil {
		return root, res.Err
	}

	err := json.Unmarshal(dagJsonOutput.Bytes(), &root)
	if err != nil {
		return root, err
	}
	return root, nil

}

// Define structs to match the JSON for
type PBHash struct {
	Slash string `json:"/"`
}

type PBLink struct {
	Hash  PBHash `json:"Hash"`
	Name  string `json:"Name"`
	Tsize int    `json:"Tsize"`
}

type PBData struct {
	Slash struct {
		Bytes string `json:"bytes"`
	} `json:"/"`
}

type PBNode struct {
	Data  PBData   `json:"Data"`
	Links []PBLink `json:"Links"`
}
