package reuseport

import (
	"net"
)

// TODO. for now, just pass it over to net.Listen/net.Dial

func listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

func dial(dialer net.Dialer, network, address string) (net.Conn, error) {
	return dialer.Dial(network, address)
}

// on windows, we just use the regular functions. sources
// vary on this-- some claim port reuse behavior is on by default
// on some windows systems. So we try. may the force be with you.
func available() bool {
	return true
}
