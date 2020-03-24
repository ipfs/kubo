package libp2p

import (
	"github.com/libp2p/go-libp2p"
)

var NatPortMap = simpleOpt(libp2p.NATPortMap())
var AutoNATService = simpleOpt(libp2p.EnableNATService())
