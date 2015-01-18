// Package loggables includes a bunch of transaltor functions for commonplace/stdlib
// objects. This is boilerplate code that shouldn't change much, and not sprinkled
// all over the place (i.e. gather it here).
//
// Note: it may make sense to put all stdlib Loggable functions in the eventlog
// package. Putting it here for now in case we don't want to polute it.
package loggables

import (
	"net"

	log "github.com/jbenet/go-ipfs/thirdparty/eventlog"
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
