package nat

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	nat "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/fd/go-nat"
	goprocess "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("nat")

const MappingDuration = time.Second * 60

func DiscoverGateway() nat.NAT {
	nat, err := nat.DiscoverGateway()
	if err != nil {
		log.Debug("DiscoverGateway error:", err)
		return nil
	}
	addr, err := nat.GetDeviceAddress()
	if err != nil {
		log.Debug("DiscoverGateway address error:", err)
	} else {
		log.Debug("DiscoverGateway address:", addr)
	}
	return nat
}

type Mapping interface {
	NAT() nat.NAT
	Protocol() string
	InternalPort() int
	ExternalPort() int
}

type mapping struct {
	// keeps republishing
	nat     nat.NAT
	proto   string
	intport int
	extport int
	proc    goprocess.Process
}

func (m *mapping) NAT() nat.NAT {
	return m.nat
}
func (m *mapping) Protocol() string {
	return m.proto
}
func (m *mapping) InternalPort() int {
	return m.intport
}
func (m *mapping) ExternalPort() int {
	return m.extport
}

// NewMapping attemps to construct a mapping on protocl and internal port
func NewMapping(nat nat.NAT, protocol string, internalPort int) (Mapping, error) {
	log.Debugf("Attempting port map: %s/%d", protocol, internalPort)
	eport, err := nat.AddPortMapping(protocol, internalPort, "http", MappingDuration)
	if err != nil {
		return nil, err
	}

	m := &mapping{
		nat:     nat,
		proto:   protocol,
		intport: internalPort,
		extport: eport,
	}

	m.proc = goprocess.Go(func(worker goprocess.Process) {
		for {
			select {
			case <-worker.Closing():
				return
			case <-time.After(MappingDuration / 3):
				eport, err := m.NAT().AddPortMapping(protocol, internalPort, "http", MappingDuration)
				if err != nil {
					log.Warningf("failed to renew port mapping: %s", err)
					continue
				}
				if eport != m.extport {
					log.Warningf("failed to renew same port mapping: ch %d -> %d", m.extport, eport)
				}
			}
		}
	})

	return m, nil
}

func (m *mapping) Close() error {
	return m.proc.Close()
}

func MapAddr(n nat.NAT, maddr ma.Multiaddr) (ma.Multiaddr, error) {
	if n == nil {
		return nil, fmt.Errorf("no nat available")
	}

	ip, err := n.GetExternalAddress()
	if err != nil {
		return nil, err
	}

	ipmaddr, err := manet.FromIP(ip)
	if err != nil {
		return nil, fmt.Errorf("error parsing ip")
	}

	network, addr, err := manet.DialArgs(maddr)
	if err != nil {
		return nil, fmt.Errorf("DialArgs failed on addr:", maddr.String())
	}

	switch network {
	case "tcp", "tcp4", "tcp6":
		network = "tcp"
	case "udp", "udp4", "udp6":
		network = "udp"
	default:
		return nil, fmt.Errorf("transport not supported by NAT: %s", network)
	}

	port := strings.Split(addr, ":")[1]
	intport, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	m, err := NewMapping(n, "tcp", intport)
	if err != nil {
		return nil, err
	}

	tcp, err := ma.NewMultiaddr(fmt.Sprintf("/tcp/%d", m.ExternalPort()))
	if err != nil {
		return nil, err
	}

	maddr2 := ipmaddr.Encapsulate(tcp)
	log.Debugf("NAT Mapping: %s --> %s", maddr, maddr2)
	return maddr2, nil
}

func MapAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	nat := DiscoverGateway()

	var advertise []ma.Multiaddr
	for _, maddr := range addrs {
		maddr2, err := MapAddr(nat, maddr)
		if err != nil || maddr2 == nil {
			log.Debug("failed to map addr:", maddr, err)
			continue
		}
		advertise = append(advertise, maddr2)
	}
	return advertise
}
