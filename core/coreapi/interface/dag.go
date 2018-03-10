package iface

import (
	"context"
	"io"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// DagAPI specifies the interface to IPLD
type DagAPI interface {
	// Put inserts data using specified format and input encoding.
	// Unless used with WithCodec or WithHash, the defaults "dag-cbor" and
	// "sha256" are used.
	Put(ctx context.Context, src io.Reader, opts ...options.DagPutOption) (Path, error)

	// WithInputEnc is an option for Put which specifies the input encoding of the
	// data. Default is "json", most formats/codecs support "raw"
	WithInputEnc(enc string) options.DagPutOption

	// WithCodec is an option for Put which specifies the multicodec to use to
	// serialize the object. Default is cid.DagCBOR (0x71)
	WithCodec(codec uint64) options.DagPutOption

	// WithHash is an option for Put which specifies the multihash settings to use
	// when hashing the object. Default is based on the codec used
	// (mh.SHA2_256 (0x12) for DagCBOR). If mhLen is set to -1, default length for
	// the hash will be used
	WithHash(mhType uint64, mhLen int) options.DagPutOption

	// Get attempts to resolve and get the node specified by the path
	Get(ctx context.Context, path Path) (ipld.Node, error)

	// Tree returns list of paths within a node specified by the path.
	Tree(ctx context.Context, path Path, opts ...options.DagTreeOption) ([]Path, error)

	// WithDepth is an option for Tree which specifies maximum depth of the
	// returned tree. Default is -1 (no depth limit)
	WithDepth(depth int) options.DagTreeOption
}
