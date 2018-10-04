// +build !linux

package sockets

import (
	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
)

// TakeSockets takes the sockets associated with the given name.
func TakeSockets(name string) ([]manet.Listener, error) {
	return nil, nil
}
