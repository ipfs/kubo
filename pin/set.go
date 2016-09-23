package pin

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"unsafe"

	"github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin/internal/pb"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	"gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
)

const (
	defaultFanout = 256
	maxItems      = 8192
)

func randomSeed() (uint32, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

func hash(seed uint32, c *cid.Cid) uint32 {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], seed)
	h := fnv.New32a()
	_, _ = h.Write(buf[:])
	_, _ = h.Write(c.Bytes())
	return h.Sum32()
}

type itemIterator func() (c *cid.Cid, data []byte, ok bool)

type keyObserver func(*cid.Cid)

// refcount is the marshaled format of refcounts. It may change
// between versions; this is valid for version 1. Changing it may
// become desirable if there are many links with refcount > 255.
//
// There are two guarantees that need to be preserved, if this is
// changed:
//
//     - the marshaled format is of fixed size, matching
//       unsafe.Sizeof(refcount(0))
//     - methods of refcount handle endianness, and may
//       in later versions need encoding/binary.
type refcount uint8

func (r refcount) Bytes() []byte {
	return []byte{byte(r)}
}

// readRefcount returns the idx'th refcount in []byte, which is
// assumed to be a sequence of refcount.Bytes results.
func (r *refcount) ReadFromIdx(buf []byte, idx int) {
	*r = refcount(buf[idx])
}

type sortByHash struct {
	links []*merkledag.Link
	data  []byte
}

func (s sortByHash) Len() int {
	return len(s.links)
}

func (s sortByHash) Less(a, b int) bool {
	return bytes.Compare(s.links[a].Hash, s.links[b].Hash) == -1
}

func (s sortByHash) Swap(a, b int) {
	s.links[a], s.links[b] = s.links[b], s.links[a]
	if len(s.data) != 0 {
		const n = int(unsafe.Sizeof(refcount(0)))
		tmp := make([]byte, n)
		copy(tmp, s.data[a*n:a*n+n])
		copy(s.data[a*n:a*n+n], s.data[b*n:b*n+n])
		copy(s.data[b*n:b*n+n], tmp)
	}
}

func storeItems(ctx context.Context, dag merkledag.DAGService, estimatedLen uint64, iter itemIterator, internalKeys keyObserver) (*merkledag.Node, error) {
	seed, err := randomSeed()
	if err != nil {
		return nil, err
	}
	n := &merkledag.Node{
		Links: make([]*merkledag.Link, 0, defaultFanout+maxItems),
	}
	for i := 0; i < defaultFanout; i++ {
		n.Links = append(n.Links, &merkledag.Link{Hash: emptyKey.Hash()})
	}
	internalKeys(emptyKey)
	hdr := &pb.Set{
		Version: proto.Uint32(1),
		Fanout:  proto.Uint32(defaultFanout),
		Seed:    proto.Uint32(seed),
	}
	if err := writeHdr(n, hdr); err != nil {
		return nil, err
	}
	hdrLen := len(n.Data())

	if estimatedLen < maxItems {
		// it'll probably fit
		for i := 0; i < maxItems; i++ {
			k, data, ok := iter()
			if !ok {
				// all done
				break
			}
			n.Links = append(n.Links, &merkledag.Link{Hash: k.Hash()})
			n.SetData(append(n.Data(), data...))
		}
		// sort by hash, also swap item Data
		s := sortByHash{
			links: n.Links[defaultFanout:],
			data:  n.Data()[hdrLen:],
		}
		sort.Stable(s)
	}

	// wasteful but simple
	type item struct {
		c    *cid.Cid
		data []byte
	}
	hashed := make(map[uint32][]item)
	for {
		k, data, ok := iter()
		if !ok {
			break
		}
		h := hash(seed, k)
		hashed[h] = append(hashed[h], item{k, data})
	}
	for h, items := range hashed {
		childIter := func() (c *cid.Cid, data []byte, ok bool) {
			if len(items) == 0 {
				return nil, nil, false
			}
			first := items[0]
			items = items[1:]
			return first.c, first.data, true
		}
		child, err := storeItems(ctx, dag, uint64(len(items)), childIter, internalKeys)
		if err != nil {
			return nil, err
		}
		size, err := child.Size()
		if err != nil {
			return nil, err
		}
		childKey, err := dag.Add(child)
		if err != nil {
			return nil, err
		}
		internalKeys(childKey)
		l := &merkledag.Link{
			Name: "",
			Hash: childKey.Hash(),
			Size: size,
		}
		n.Links[int(h%defaultFanout)] = l
	}
	return n, nil
}

func readHdr(n *merkledag.Node) (*pb.Set, []byte, error) {
	hdrLenRaw, consumed := binary.Uvarint(n.Data())
	if consumed <= 0 {
		return nil, nil, errors.New("invalid Set header length")
	}
	buf := n.Data()[consumed:]
	if hdrLenRaw > uint64(len(buf)) {
		return nil, nil, errors.New("impossibly large Set header length")
	}
	// as hdrLenRaw was <= an int, we now know it fits in an int
	hdrLen := int(hdrLenRaw)
	var hdr pb.Set
	if err := proto.Unmarshal(buf[:hdrLen], &hdr); err != nil {
		return nil, nil, err
	}
	buf = buf[hdrLen:]

	if v := hdr.GetVersion(); v != 1 {
		return nil, nil, fmt.Errorf("unsupported Set version: %d", v)
	}
	if uint64(hdr.GetFanout()) > uint64(len(n.Links)) {
		return nil, nil, errors.New("impossibly large Fanout")
	}
	return &hdr, buf, nil
}

func writeHdr(n *merkledag.Node, hdr *pb.Set) error {
	hdrData, err := proto.Marshal(hdr)
	if err != nil {
		return err
	}
	n.SetData(make([]byte, binary.MaxVarintLen64, binary.MaxVarintLen64+len(hdrData)))
	written := binary.PutUvarint(n.Data(), uint64(len(hdrData)))
	n.SetData(n.Data()[:written])
	n.SetData(append(n.Data(), hdrData...))
	return nil
}

type walkerFunc func(buf []byte, idx int, link *merkledag.Link) error

func walkItems(ctx context.Context, dag merkledag.DAGService, n *merkledag.Node, fn walkerFunc, children keyObserver) error {
	hdr, buf, err := readHdr(n)
	if err != nil {
		return err
	}
	// readHdr guarantees fanout is a safe value
	fanout := hdr.GetFanout()
	for i, l := range n.Links[fanout:] {
		if err := fn(buf, i, l); err != nil {
			return err
		}
	}
	for _, l := range n.Links[:fanout] {
		c := cid.NewCidV0(l.Hash)
		children(c)
		if c.Equals(emptyKey) {
			continue
		}
		subtree, err := l.GetNode(ctx, dag)
		if err != nil {
			return err
		}
		if err := walkItems(ctx, dag, subtree, fn, children); err != nil {
			return err
		}
	}
	return nil
}

func loadSet(ctx context.Context, dag merkledag.DAGService, root *merkledag.Node, name string, internalKeys keyObserver) ([]*cid.Cid, error) {
	l, err := root.GetNodeLink(name)
	if err != nil {
		return nil, err
	}

	lnkc := cid.NewCidV0(l.Hash)
	internalKeys(lnkc)

	n, err := l.GetNode(ctx, dag)
	if err != nil {
		return nil, err
	}

	var res []*cid.Cid
	walk := func(buf []byte, idx int, link *merkledag.Link) error {
		res = append(res, cid.NewCidV0(link.Hash))
		return nil
	}
	if err := walkItems(ctx, dag, n, walk, internalKeys); err != nil {
		return nil, err
	}
	return res, nil
}

func loadMultiset(ctx context.Context, dag merkledag.DAGService, root *merkledag.Node, name string, internalKeys keyObserver) (map[key.Key]uint64, error) {
	l, err := root.GetNodeLink(name)
	if err != nil {
		return nil, fmt.Errorf("Failed to get link %s: %v", name, err)
	}
	c := cid.NewCidV0(l.Hash)
	internalKeys(c)
	n, err := l.GetNode(ctx, dag)
	if err != nil {
		return nil, fmt.Errorf("Failed to get node from link %s: %v", name, err)
	}

	refcounts := make(map[key.Key]uint64)
	walk := func(buf []byte, idx int, link *merkledag.Link) error {
		var r refcount
		r.ReadFromIdx(buf, idx)
		refcounts[key.Key(link.Hash)] += uint64(r)
		return nil
	}
	if err := walkItems(ctx, dag, n, walk, internalKeys); err != nil {
		return nil, err
	}
	return refcounts, nil
}

func storeSet(ctx context.Context, dag merkledag.DAGService, cids []*cid.Cid, internalKeys keyObserver) (*merkledag.Node, error) {
	iter := func() (c *cid.Cid, data []byte, ok bool) {
		if len(cids) == 0 {
			return nil, nil, false
		}
		first := cids[0]
		cids = cids[1:]
		return first, nil, true
	}
	n, err := storeItems(ctx, dag, uint64(len(cids)), iter, internalKeys)
	if err != nil {
		return nil, err
	}
	c, err := dag.Add(n)
	if err != nil {
		return nil, err
	}
	internalKeys(c)
	return n, nil
}

func copyRefcounts(orig map[key.Key]uint64) map[key.Key]uint64 {
	r := make(map[key.Key]uint64, len(orig))
	for k, v := range orig {
		r[k] = v
	}
	return r
}
