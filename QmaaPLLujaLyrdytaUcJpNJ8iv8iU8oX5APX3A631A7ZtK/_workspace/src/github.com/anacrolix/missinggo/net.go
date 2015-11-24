package missinggo

import (
	"net"
)

type HostPort struct {
	Host string // Just the host, with no port.
	Port string // May be empty if no port was given.
	Err  error  // The error returned from net.SplitHostPort.
}

// Parse a "hostport" string, a concept that floats around the stdlib a lot
// and is painful to work with. If no port is present, what's usually present
// is just the host.
func ParseHostPort(hostPort string) (ret HostPort) {
	ret.Host, ret.Port, ret.Err = net.SplitHostPort(hostPort)
	if ret.Err != nil {
		ret.Host = hostPort
	}
	return
}

func (me *HostPort) Join() string {
	return net.JoinHostPort(me.Host, me.Port)
}
