package dht

// A helper struct to make working with protbuf types easier
type pDHTMessage struct {
	Type DHTMessage_MessageType
	Key string
	Value []byte
	Response bool
	Id uint64
}

func (m *pDHTMessage) ToProtobuf() *DHTMessage {
	pmes := new(DHTMessage)
	if m.Value != nil {
		pmes.Value = m.Value
	}

	pmes.Type = &m.Type
	pmes.Key = &m.Key
	pmes.Response = &m.Response
	pmes.Id = &m.Id

	return pmes
}
