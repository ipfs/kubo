package bitswap

import (
	"code.google.com/p/goprotobuf/proto"
	u "github.com/jbenet/go-ipfs/util"
)

type Message struct {
	ID       uint64
	Response bool
	Key      u.Key
	Value    []byte
	Success  bool
}

func (m *Message) ToProtobuf() *PBMessage {
	pmes := new(PBMessage)
	pmes.Id = &m.ID
	if m.Response {
		pmes.Response = proto.Bool(true)
	}

	if m.Success {
		pmes.Success = proto.Bool(true)
	}

	pmes.Key = proto.String(string(m.Key))
	pmes.Value = m.Value
	return pmes
}
