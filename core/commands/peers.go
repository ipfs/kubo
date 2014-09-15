package commands

import (
  "encoding/json"
  "io"

  "github.com/jbenet/go-ipfs/core"
  b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
)

// peerInfo is a representation of a peer.Peer that is better suited for serialization
type peerInfo struct {
  ID string
  Addresses []string
  PubKey string
  Latency int64
}

func Peers(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
  enc := json.NewEncoder(out)

  i := 0
  peers := make([]*peerInfo, len(*n.PeerMap))

  for _, p := range *n.PeerMap {
    addrs := make([]string, len(p.Addresses))
    for i, addr := range p.Addresses {
      addrStr, err := addr.String()
      if err != nil {
        return err
      }
      addrs[i] = addrStr
    }

    pubkeyBytes, err := p.PubKey.Bytes()
    if err != nil {
      return err
    }

    peer := &peerInfo{
      ID: p.ID.Pretty(),
      Addresses: addrs,
      PubKey: b58.Encode(pubkeyBytes),
      Latency: p.GetLatency().Nanoseconds(),
    }
    peers[i] = peer
    i++
  }

  enc.Encode(peers)

  return nil
}
