module github.com/ipfs/go-ipfs/examples/go-ipfs-as-a-library

go 1.16

require (
	github.com/ipfs/go-ipfs v0.9.1
	github.com/ipfs/go-ipfs-config v0.16.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/interface-go-ipfs-core v0.5.1
	github.com/libp2p/go-libp2p-core v0.9.0
	github.com/multiformats/go-multiaddr v0.4.0
)

replace github.com/ipfs/go-ipfs => ./../../..
