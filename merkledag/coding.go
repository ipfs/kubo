package merkledag

import (
	"fmt"
	"sort"
	"strings"

	"gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"

	pb "github.com/ipfs/go-ipfs/merkledag/pb"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
	"sync"
)

// for now, we use a PBNode intermediate thing.
// because native go objects are nice.

// unmarshal decodes raw data into a *Node instance.
// The conversion uses an intermediate PBNode.
func (n *ProtoNode) unmarshal(encoded []byte) error {
	var pbn pb.PBNode
	if err := pbn.Unmarshal(encoded); err != nil {
		return fmt.Errorf("Unmarshal failed. %v", err)
	}

	pbnl := pbn.GetLinks()
	n.links = make([]*ipld.Link, len(pbnl))
	for i, l := range pbnl {
		n.links[i] = &ipld.Link{Name: l.GetName(), Size: l.GetTsize()}
		c, err := cid.Cast(l.GetHash())
		if err != nil {
			return fmt.Errorf("Link hash #%d is not valid multihash. %v", i, err)
		}
		n.links[i].Cid = c
	}
	sort.Stable(LinkSlice(n.links)) // keep links sorted

	n.data = pbn.GetData()
	n.setEncodedValue(encoded)
	return nil
}

// marshal encodes a *Node instance into a new byte slice.
// The conversion uses an intermediate PBNode.
func (n *ProtoNode) marshal() ([]byte, error) {
	pbn := n.getPBNode()
	data, err := pbn.Marshal()
	if err != nil {
		return data, fmt.Errorf("Marshal failed. %v", err)
	}
	return data, nil
}

func (n *ProtoNode) getPBNode() *pb.PBNode {
	pbn := &pb.PBNode{}
	if len(n.links) > 0 {
		pbn.Links = make([]*pb.PBLink, len(n.links))
	}

	sort.Stable(LinkSlice(n.links)) // keep links sorted
	for i, l := range n.links {
		pbn.Links[i] = &pb.PBLink{}
		pbn.Links[i].Name = &l.Name
		pbn.Links[i].Tsize = &l.Size
		if l.Cid != nil {
			pbn.Links[i].Hash = l.Cid.Bytes()
		}
	}

	if len(n.data) > 0 {
		pbn.Data = n.data
	}
	return pbn
}

// setEncodedValue is the single gateway to modify the encoded node.
func (n *ProtoNode) setEncodedValue(value []byte) {
	n.cache.encodedValue = value

	// The encoding has changed so the cached CID is invalid.
	n.cache.cid = nil
}

// invalidateCache sets the encoding to nil (invalidating
// also the cached CID).
func (n *ProtoNode) invalidateCache() {
	n.setEncodedValue(nil)
	// Recreate the lock allowing the cache to be (re)initialized
	// in the next call to `EncodeProtobuf`.
	n.cache.initialize = sync.Once{}
}

// EncodeProtobuf returns the encoded raw data version of a Node instance.
func (n *ProtoNode) EncodeProtobuf() ([]byte, error) {
	n.cache.initialize.Do(func() {
		sort.Stable(LinkSlice(n.links)) // keep links sorted
		marshaledNode, err := n.marshal()
		if err != nil {
			n.invalidateCache()
			n.cache.initializationError = err
		} else {
			n.setEncodedValue(marshaledNode)
		}
	})

	return n.cache.encodedValue, n.cache.initializationError
}

// DecodeProtobuf decodes raw data and returns a new Node instance.
func DecodeProtobuf(encoded []byte) (*ProtoNode, error) {
	n := new(ProtoNode)
	err := n.unmarshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("incorrectly formatted merkledag node: %s", err)
	}
	return n, nil
}

// DecodeProtobufBlock is a block decoder for protobuf IPLD nodes conforming to
// node.DecodeBlockFunc
func DecodeProtobufBlock(b blocks.Block) (ipld.Node, error) {
	c := b.Cid()
	if c.Type() != cid.DagProtobuf {
		return nil, fmt.Errorf("this function can only decode protobuf nodes")
	}

	decnd, err := DecodeProtobuf(b.RawData())
	if err != nil {
		if strings.Contains(err.Error(), "Unmarshal failed") {
			return nil, fmt.Errorf("The block referred to by '%s' was not a valid merkledag node", c)
		}
		return nil, fmt.Errorf("Failed to decode Protocol Buffers: %v", err)
	}

	decnd.cache.cid = c
	decnd.Prefix = c.Prefix()
	return decnd, nil
}

// Type assertion
var _ ipld.DecodeBlockFunc = DecodeProtobufBlock
