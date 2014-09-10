package namesys

import (
	"net"
	"strings"

	u "github.com/jbenet/go-ipfs/util"
)

type DNSResolver struct {
	// TODO: maybe some sort of caching?
	// cache would need a timeout
}

func (r *DNSResolver) Resolve(name string) (string, error) {
	txt, err := net.LookupTXT(name)
	if err != nil {
		return "", err
	}

	for _, t := range txt {
		pair := strings.Split(t, "=")
		if len(pair) < 2 {
			// Log error?
			u.DErr("Incorrectly formatted text record.")
			continue
		}
		if pair[0] == name {
			return pair[1], nil
		}
	}
	return "", u.ErrNotFound
}
