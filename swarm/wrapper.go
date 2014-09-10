package swarm

import "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

func Wrap(data []byte, typ PBWrapper_MessageType) ([]byte, error) {
	wrapper := new(PBWrapper)
	wrapper.Message = data
	wrapper.Type = &typ
	b, err := proto.Marshal(wrapper)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func Unwrap(data []byte) (*PBWrapper, error) {
	mes := new(PBWrapper)
	err := proto.Unmarshal(data, mes)
	if err != nil {
		return nil, err
	}

	return mes, nil
}
