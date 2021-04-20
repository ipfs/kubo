module github.com/ipfs/go-ipfs/examples/smart-records

go 1.14

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-config v0.12.1-0.20210415065829-ab423490da8d
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/libp2p/go-smart-record v0.0.0-20210419163011-3856ac29a69b
	github.com/multiformats/go-multiaddr v0.3.1
)

replace github.com/ipfs/go-ipfs => ./../../..
