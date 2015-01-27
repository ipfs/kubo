package nat

import (
	"net"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jackpal/go-nat-pmp"
)

var (
	_ NAT = (*natpmp_NAT)(nil)
)

func natpmp_PotentialGateways() ([]net.IP, error) {
	_, ipNet_10, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		panic(err)
	}

	_, ipNet_172_16, err := net.ParseCIDR("172.16.0.0/12")
	if err != nil {
		panic(err)
	}

	_, ipNet_192_168, err := net.ParseCIDR("192.168.0.0/16")
	if err != nil {
		panic(err)
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var ips []net.IP

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			switch x := addr.(type) {
			case *net.IPNet:
				var ipNet *net.IPNet
				if ipNet_10.Contains(x.IP) {
					ipNet = ipNet_10
				} else if ipNet_172_16.Contains(x.IP) {
					ipNet = ipNet_172_16
				} else if ipNet_192_168.Contains(x.IP) {
					ipNet = ipNet_192_168
				}

				if ipNet != nil {
					ip := x.IP.Mask(x.Mask)
					ip = ip.To4()
					if ip != nil {
						ip[3] = ip[3] | 0x01
						ips = append(ips, ip)
					}
				}
			}
		}
	}

	if len(ips) == 0 {
		return nil, ErrNoNATFound
	}

	return ips, nil
}

func discoverNATPMP() <-chan NAT {
	ips, err := natpmp_PotentialGateways()
	if err != nil {
		return nil
	}

	res := make(chan NAT, len(ips))

	for _, ip := range ips {
		go discoverNATPMPWithAddr(res, ip)
	}

	return res
}

func discoverNATPMPWithAddr(c chan NAT, ip net.IP) {
	client := natpmp.NewClient(ip)
	_, err := client.GetExternalAddress()
	if err != nil {
		return
	}

	c <- &natpmp_NAT{client, ip, make(map[int]int)}
}

type natpmp_NAT struct {
	c       *natpmp.Client
	gateway net.IP
	ports   map[int]int
}

func (n *natpmp_NAT) GetDeviceAddress() (addr net.IP, err error) {
	return n.gateway, nil
}

func (n *natpmp_NAT) GetInternalAddress() (addr net.IP, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			switch x := addr.(type) {
			case *net.IPNet:
				if x.Contains(n.gateway) {
					return x.IP, nil
				}
			}
		}
	}

	return nil, ErrNoInternalAddress
}

func (n *natpmp_NAT) GetExternalAddress() (addr net.IP, err error) {
	res, err := n.c.GetExternalAddress()
	if err != nil {
		return nil, err
	}

	d := res.ExternalIPAddress
	return net.IPv4(d[0], d[1], d[2], d[3]), nil
}

func (n *natpmp_NAT) AddPortMapping(protocol string, internalPort int, description string, timeout time.Duration) (int, error) {
	var (
		err error
	)

	timeoutInSeconds := int(timeout / time.Second)

	if externalPort := n.ports[internalPort]; externalPort > 0 {
		_, err = n.c.AddPortMapping(protocol, internalPort, externalPort, timeoutInSeconds)
		if err == nil {
			n.ports[internalPort] = externalPort
			return externalPort, nil
		}
	}

	for i := 0; i < 3; i++ {
		externalPort := randomPort()
		_, err = n.c.AddPortMapping(protocol, internalPort, externalPort, timeoutInSeconds)
		if err == nil {
			n.ports[internalPort] = externalPort
			return externalPort, nil
		}
	}

	return 0, err
}

func (n *natpmp_NAT) DeletePortMapping(protocol string, internalPort int) (err error) {
	delete(n.ports, internalPort)
	return nil
}

func (n *natpmp_NAT) Type() string {
	return "NAT-PMP"
}
