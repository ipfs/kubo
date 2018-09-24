// +build !linux

package sockets

import (
	manet "github.com/multiformats/go-multiaddr-net"
)

// TakeSockets takes the sockets associated with the given name.
func TakeSockets(name string) ([]manet.Listener, error) {
	return nil, nil
}
