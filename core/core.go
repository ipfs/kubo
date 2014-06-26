package core

// IPFS Core module. It represents an IPFS instance.

type IPFSNode struct {

  // the local node's identity (a Peer instance)
  Identity *peer.Peer

  // the book of other nodes (a hashtable of Peer instances)
  PeerBook *peerbook.PeerBook

  // the local database (uses datastore)
  Storage *storage.Storage

  // the network message stream
  Network *netmux.Netux

  // the routing system. recommend ipfs-dht
  Routing *routing.Routing

  // the block exchange + strategy (bitswap)
  BitSwap *bitswap.BitSwap

  // the block service, get/add blocks.
  Blocks *blocks.BlockService

  // the path resolution system
  Resolver *resolver.PathResolver

  // the name system, resolves paths to hashes
  Namesys *namesys.Namesys
}
