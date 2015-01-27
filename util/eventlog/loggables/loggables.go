// Package loggables includes a bunch of transaltor functions for commonplace/stdlib
// objects. This is boilerplate code that shouldn't change much, and not sprinkled
// all over the place (i.e. gather it here).
//
// Note: it may make sense to put all stdlib Loggable functions in the eventlog
// package. Putting it here for now in case we don't want to polute it.
package loggables

import (
	"net"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	log "github.com/jbenet/go-ipfs/thirdparty/eventlog"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
)

// NetConn returns an eventlog.Metadata with the conn addresses
func NetConn(c net.Conn) log.Loggable {
	return log.Metadata{
		"localAddr":  c.LocalAddr(),
		"remoteAddr": c.RemoteAddr(),
	}
}

// Error returns an eventlog.Metadata with an error
func Error(e error) log.Loggable {
	return log.Metadata{
		"error": e.Error(),
	}
}

// Dial metadata is metadata for dial events
func Dial(sys string, lid, rid peer.ID, laddr, raddr ma.Multiaddr) log.LoggableMap {
	m := log.Metadata{"subsystem": sys}
	if lid != "" {
		m["localPeer"] = lid.Pretty()
	}
	if laddr != nil {
		m["localAddr"] = laddr.String()
	}
	if rid != "" {
		m["remotePeer"] = rid.Pretty()
	}
	if raddr != nil {
		m["remoteAddr"] = raddr.String()
	}
	return log.LoggableMap(m)
}
