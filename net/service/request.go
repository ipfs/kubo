package service

import (
	crand "crypto/rand"

	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"

	proto "code.google.com/p/goprotobuf/proto"
)

const (
	// IDSize is the size of the ID in bytes.
	IDSize int = 4
)

// RequestID is a field that identifies request-response flows.
type RequestID []byte

// Request turns a RequestID into a Request (unsetting first bit)
func (r RequestID) Request() RequestID {
	if r == nil {
		return nil
	}
	r2 := make([]byte, len(r))
	copy(r2, r)
	r2[0] = r[0] & 0x7F // unset first bit for request
	return RequestID(r2)
}

// Response turns a RequestID into a Response (setting first bit)
func (r RequestID) Response() RequestID {
	if r == nil {
		return nil
	}
	r2 := make([]byte, len(r))
	copy(r2, r)
	r2[0] = r[0] | 0x80 // set first bit for response
	return RequestID(r2)
}

// IsRequest returns whether a RequestID identifies a request
func (r RequestID) IsRequest() bool {
	if r == nil {
		return false
	}
	return !r.IsResponse()
}

// IsResponse returns whether a RequestID identifies a response
func (r RequestID) IsResponse() bool {
	if r == nil {
		return false
	}
	return bool(r[0]&0x80 == 0x80)
}

// RandomRequestID creates and returns a new random request ID
func RandomRequestID() (RequestID, error) {
	buf := make([]byte, IDSize)
	_, err := crand.Read(buf)
	return RequestID(buf).Request(), err
}

// RequestMap is a map of Requests. the key = (peer.ID concat RequestID).
type RequestMap map[string]*Request

// Request objects are used to multiplex request-response flows.
type Request struct {

	// ID is the RequestID identifying this Request-Response Flow.
	ID RequestID

	// PeerID identifies the peer from whom to expect the response.
	PeerID peer.ID

	// Response is the channel of incoming responses.
	Response chan msg.NetMessage
}

// NewRequest creates a request for given peer.ID
func NewRequest(pid peer.ID) (*Request, error) {
	id, err := RandomRequestID()
	if err != nil {
		return nil, err
	}

	return &Request{
		ID:       id,
		PeerID:   pid,
		Response: make(chan msg.NetMessage, 1),
	}, nil
}

// Key returns the RequestKey for this request. Use with maps.
func (r *Request) Key() string {
	return RequestKey(r.PeerID, r.ID)
}

// RequestKey is the peer.ID concatenated with the RequestID. Use with maps.
func RequestKey(pid peer.ID, rid RequestID) string {
	return string(pid) + string(rid.Request()[:])
}

func wrapData(data []byte, rid RequestID) ([]byte, error) {
	// Marshal
	pbm := new(PBRequest)
	pbm.Data = data
	pbm.Tag = rid
	b, err := proto.Marshal(pbm)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func unwrapData(data []byte) ([]byte, RequestID, error) {
	// Unmarshal
	pbm := new(PBRequest)
	err := proto.Unmarshal(data, pbm)
	if err != nil {
		return nil, nil, err
	}

	return pbm.GetData(), pbm.GetTag(), nil
}
