package mask

import (
	"errors"
	"fmt"
	"net"
	"strings"

	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
)

var ErrInvalidFormat = errors.New("invalid multiaddr-filter format")

func NewMask(a string) (*net.IPNet, error) {
	parts := strings.Split(a, "/")

	if parts[0] != "" {
		return nil, ErrInvalidFormat
	}

	if len(parts) != 5 {
		return nil, ErrInvalidFormat
	}

	// check it's a valid filter address. ip + cidr
	isip := parts[1] == "ip4" || parts[1] == "ip6"
	iscidr := parts[3] == "ipcidr"
	if !isip || !iscidr {
		return nil, ErrInvalidFormat
	}

	_, ipn, err := net.ParseCIDR(parts[2] + "/" + parts[4])
	if err != nil {
		return nil, err
	}
	return ipn, nil
}

func ConvertIPNet(n *net.IPNet) (string, error) {
	addr, err := manet.FromIP(n.IP)
	if err != nil {
		return "", err
	}

	b, _ := n.Mask.Size()
	return fmt.Sprintf("%s/ipcidr/%d", addr, b), nil
}
