package dht

// A helper struct to make working with protbuf types easier
type pDHTMessage struct {
	Type DHTMessage_MessageType
	Key string
	Value []byte
	Response bool
	Id uint64
}

var mesNames [10]string

func init() {
	mesNames[DHTMessage_ADD_PROVIDER] = "add provider"
	mesNames[DHTMessage_FIND_NODE] = "find node"
	mesNames[DHTMessage_GET_PROVIDERS] = "get providers"
	mesNames[DHTMessage_GET_VALUE] = "get value"
	mesNames[DHTMessage_PUT_VALUE] = "put value"
	mesNames[DHTMessage_PING] = "ping"
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
