package mask

import (
	"errors"
	"net"
	"strings"
)

func NewMask(a string) (*net.IPNet, error) {
	parts := strings.Split(a, "/")
	if len(parts) == 5 && parts[1] == "ip4" && parts[3] == "ipcidr" {
		_, ipn, err := net.ParseCIDR(parts[2] + "/" + parts[4])
		if err != nil {
			return nil, err
		}
		return ipn, nil
	}
	return nil, errors.New("invalid format")
}
