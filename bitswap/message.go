package bitswap

import (
	"code.google.com/p/goprotobuf/proto"
	u "github.com/jbenet/go-ipfs/util"
)

type Message struct {
	Type     PBMessage_MessageType
	ID       uint64
	Response bool
	Key      u.Key
	Value    []byte
	Success  bool
	WantList KeySet
}

func (m *Message) ToProtobuf() *PBMessage {
	pmes := new(PBMessage)
	pmes.Id = &m.ID
	pmes.Type = &m.Type
	if m.Response {
		pmes.Response = proto.Bool(true)
	}

	if m.Success {
		pmes.Success = proto.Bool(true)
	}

	if m.WantList != nil {
		var swant []string
		for k, _ := range m.WantList {
			swant = append(swant, string(k))
		}
		pmes.Wantlist = swant
	}

	pmes.Key = proto.String(string(m.Key))
	pmes.Value = m.Value
	return pmes
}
