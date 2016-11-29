package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"

	rtable "gx/ipfs/QmRVHVr38ChANF2PUMNKQs7Q4uVWCLVabrfcTG9taNbcVy/go-libp2p-kbucket"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
)

type node struct {
	Name  string `json:"name"`
	Value uint64 `json:"value"`
	RtKey string `json:"rtkey"`
}

type link struct {
	Source int `json:"source"`
	Target int `json:"target"`
	Value  int `json:"value"`
}

func GetGraphJson(dinfo []*DiagInfo) []byte {
	out := make(map[string]interface{})
	names := make(map[string]int)
	var nodes []*node
	for _, di := range dinfo {
		names[di.ID] = len(nodes)
		val := di.BwIn + di.BwOut + 10
		// include the routing table key, for proper routing table display
		rtk := peer.ID(rtable.ConvertPeerID(peer.ID(di.ID))).Pretty()
		nodes = append(nodes, &node{Name: di.ID, Value: val, RtKey: rtk})
	}

	var links []*link
	linkexists := make([][]bool, len(nodes))
	for i := range linkexists {
		linkexists[i] = make([]bool, len(nodes))
	}

	for _, di := range dinfo {
		myid := names[di.ID]
		for _, con := range di.Connections {
			thisid := names[con.ID]
			if !linkexists[thisid][myid] {
				links = append(links, &link{
					Source: myid,
					Target: thisid,
					Value:  3,
				})
				linkexists[myid][thisid] = true
			}
		}
	}

	out["nodes"] = nodes
	out["links"] = links

	b, err := json.Marshal(out)
	if err != nil {
		panic(err)
	}

	return b
}

type DotWriter struct {
	W   io.Writer
	err error
}

// Write writes a buffer to the internal writer.
// It handles errors as in: http://blog.golang.org/errors-are-values
func (w *DotWriter) Write(buf []byte) (n int, err error) {
	if w.err == nil {
		n, w.err = w.W.Write(buf)
	}
	return n, w.err
}

// WriteS writes a string
func (w *DotWriter) WriteS(s string) (n int, err error) {
	return w.Write([]byte(s))
}

func (w *DotWriter) WriteNetHeader(dinfo []*DiagInfo) error {
	label := fmt.Sprintf("Nodes: %d\\l", len(dinfo))

	w.WriteS("subgraph cluster_L { ")
	w.WriteS("L [shape=box fontsize=32 label=\"" + label + "\"] ")
	w.WriteS("}\n")
	return w.err
}

func (w *DotWriter) WriteNode(i int, di *DiagInfo) error {
	box := "[label=\"%s\n%d conns\" fontsize=8 shape=box tooltip=\"%s (%d conns)\"]"
	box = fmt.Sprintf(box, di.ID, len(di.Connections), di.ID, len(di.Connections))

	w.WriteS(fmt.Sprintf("N%d %s\n", i, box))
	return w.err
}

func (w *DotWriter) WriteEdge(i, j int, di *DiagInfo, conn connDiagInfo) error {

	n := fmt.Sprintf("%s ... %s (%d)", di.ID, conn.ID, conn.Latency)
	s := "[label=\" %d\" weight=%d tooltip=\"%s\" labeltooltip=\"%s\" style=\"dotted\"]"
	s = fmt.Sprintf(s, conn.Latency, conn.Count, n, n)

	w.WriteS(fmt.Sprintf("N%d -> N%d %s\n", i, j, s))
	return w.err
}

func (w *DotWriter) WriteGraph(dinfo []*DiagInfo) error {
	w.WriteS("digraph \"diag-net\" {\n")
	w.WriteNetHeader(dinfo)

	idx := make(map[string]int)
	for i, di := range dinfo {
		if _, found := idx[di.ID]; found {
			log.Debugf("DotWriter skipped duplicate %s", di.ID)
			continue
		}

		idx[di.ID] = i
		w.WriteNode(i, di)
	}

	for i, di := range dinfo {
		for _, conn := range di.Connections {
			j, found := idx[conn.ID]
			if !found { // if we didnt get it earlier...
				j = len(idx)
				idx[conn.ID] = j
			}

			w.WriteEdge(i, j, di, conn)
		}
	}

	w.WriteS("}")
	return w.err
}
