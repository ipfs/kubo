package mask

import (
	"net"
	"testing"
)

func TestFiltered(t *testing.T) {
	var tests = map[string]map[string]bool{
		"/ip4/10.0.0.0/ipcidr/8": map[string]bool{
			"10.3.3.4":   true,
			"10.3.4.4":   true,
			"10.4.4.4":   true,
			"15.52.34.3": false,
		},
		"/ip4/192.168.0.0/ipcidr/16": map[string]bool{
			"192.168.0.0": true,
			"192.168.1.0": true,
			"192.1.0.0":   false,
			"10.4.4.4":    false,
		},
	}

	for mask, set := range tests {
		m, err := NewMask(mask)
		if err != nil {
			t.Fatal(err)
		}
		for addr, val := range set {
			ip := net.ParseIP(addr)
			if m.Contains(ip) != val {
				t.Fatalf("expected contains(%s, %s) == %s", mask, addr, val)
			}
		}
	}
}
