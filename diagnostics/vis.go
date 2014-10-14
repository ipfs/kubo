package diagnostics

import "encoding/json"

type node struct {
	Name  string `json:"name"`
	Value uint64 `json:"value"`
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
		val := di.BwIn + di.BwOut
		nodes = append(nodes, &node{Name: di.ID, Value: val})
	}

	var links []*link
	linkexists := make([][]bool, len(nodes))
	for i, _ := range linkexists {
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
