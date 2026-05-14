package harness

import (
	"bytes"
	"encoding/json"

	mdag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	pb "github.com/ipfs/boxo/ipld/unixfs/pb"
)

// UnixFSDataType returns the UnixFS DataType for the given CID by fetching the
// raw block and parsing the protobuf. This directly checks the Type field in
// the UnixFS Data message (https://specs.ipfs.tech/unixfs/#data).
//
// Common types:
//   - ft.TDirectory (1) = basic flat directory
//   - ft.THAMTShard (5) = HAMT sharded directory
func (n *Node) UnixFSDataType(cid string) (pb.Data_DataType, error) {
	log.Debugf("node %d block get %s", n.ID, cid)

	var blockData bytes.Buffer
	res := n.Runner.MustRun(RunRequest{
		Path:    n.IPFSBin,
		Args:    []string{"block", "get", cid},
		CmdOpts: []CmdOpt{RunWithStdout(&blockData)},
	})
	if res.Err != nil {
		return 0, res.Err
	}

	// Parse dag-pb block
	protoNode, err := mdag.DecodeProtobuf(blockData.Bytes())
	if err != nil {
		return 0, err
	}

	// Parse UnixFS data
	fsNode, err := ft.FSNodeFromBytes(protoNode.Data())
	if err != nil {
		return 0, err
	}

	return fsNode.Type(), nil
}

// UnixFSHAMTFanout returns the fanout value for a HAMT shard directory.
// This is only valid for HAMT shards (THAMTShard type).
func (n *Node) UnixFSHAMTFanout(cid string) (uint64, error) {
	log.Debugf("node %d block get %s for fanout", n.ID, cid)

	var blockData bytes.Buffer
	res := n.Runner.MustRun(RunRequest{
		Path:    n.IPFSBin,
		Args:    []string{"block", "get", cid},
		CmdOpts: []CmdOpt{RunWithStdout(&blockData)},
	})
	if res.Err != nil {
		return 0, res.Err
	}

	// Parse dag-pb block
	protoNode, err := mdag.DecodeProtobuf(blockData.Bytes())
	if err != nil {
		return 0, err
	}

	// Parse UnixFS data
	fsNode, err := ft.FSNodeFromBytes(protoNode.Data())
	if err != nil {
		return 0, err
	}

	return fsNode.Fanout(), nil
}

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
