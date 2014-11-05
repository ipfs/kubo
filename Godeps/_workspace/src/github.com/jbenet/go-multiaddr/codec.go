package multiaddr

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func stringToBytes(s string) ([]byte, error) {

	// consume trailing slashes
	s = strings.TrimRight(s, "/")

	b := []byte{}
	sp := strings.Split(s, "/")

	if sp[0] != "" {
		return nil, fmt.Errorf("invalid multiaddr, must begin with /")
	}

	// consume first empty elem
	sp = sp[1:]

	for len(sp) > 0 {
		p := ProtocolWithName(sp[0])
		if p == nil {
			return nil, fmt.Errorf("no protocol with name %s", sp[0])
		}
		b = append(b, byte(p.Code))

		a := addressStringToBytes(p, sp[1])
		b = append(b, a...)

		sp = sp[2:]
	}
	return b, nil
}

func bytesToString(b []byte) (ret string, err error) {
	// panic handler, in case we try accessing bytes incorrectly.
	defer func() {
		if e := recover(); e != nil {
			ret = ""
			err = e.(error)
		}
	}()

	s := ""

	for len(b) > 0 {
		p := ProtocolWithCode(int(b[0]))
		if p == nil {
			return "", fmt.Errorf("no protocol with code %d", b[0])
		}
		s = strings.Join([]string{s, "/", p.Name}, "")
		b = b[1:]

		a := addressBytesToString(p, b[:(p.Size/8)])
		if len(a) > 0 {
			s = strings.Join([]string{s, "/", a}, "")
		}
		b = b[(p.Size / 8):]
	}

	return s, nil
}

func bytesSplit(b []byte) (ret [][]byte, err error) {
	// panic handler, in case we try accessing bytes incorrectly.
	defer func() {
		if e := recover(); e != nil {
			ret = [][]byte{}
			err = e.(error)
		}
	}()

	ret = [][]byte{}
	for len(b) > 0 {
		p := ProtocolWithCode(int(b[0]))
		if p == nil {
			return [][]byte{}, fmt.Errorf("no protocol with code %d", b[0])
		}

		length := 1 + (p.Size / 8)
		ret = append(ret, b[:length])
		b = b[length:]
	}

	return ret, nil
}

func addressStringToBytes(p *Protocol, s string) []byte {
	switch p.Code {

	case P_IP4: // ipv4
		return net.ParseIP(s).To4()

	case P_IP6: // ipv6
		return net.ParseIP(s).To16()

	// tcp udp dccp sctp
	case P_TCP, P_UDP, P_DCCP, P_SCTP:
		b := make([]byte, 2)
		i, err := strconv.Atoi(s)
		if err == nil {
			binary.BigEndian.PutUint16(b, uint16(i))
		}
		return b
	}

	return []byte{}
}

func addressBytesToString(p *Protocol, b []byte) string {
	switch p.Code {

	// ipv4,6
	case P_IP4, P_IP6:
		return net.IP(b).String()

	// tcp udp dccp sctp
	case P_TCP, P_UDP, P_DCCP, P_SCTP:
		i := binary.BigEndian.Uint16(b)
		return strconv.Itoa(int(i))
	}

	return ""
}
