package multiaddr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
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
		if p.Code == 0 {
			return nil, fmt.Errorf("no protocol with name %s", sp[0])
		}
		b = append(b, CodeToVarint(p.Code)...)
		sp = sp[1:]

		if p.Size == 0 { // no length.
			continue
		}

		if len(sp) < 1 {
			return nil, fmt.Errorf("protocol requires address, none given: %s", p.Name)
		}
		a, err := addressStringToBytes(p, sp[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %s %s", p.Name, sp[0], err)
		}
		b = append(b, a...)
		sp = sp[1:]
	}
	return b, nil
}

func bytesToString(b []byte) (ret string, err error) {
	// panic handler, in case we try accessing bytes incorrectly.
	defer func() {
		if e := recover(); e != nil {
			ret = ""
			switch e := e.(type) {
			case error:
				err = e
			case string:
				err = errors.New(e)
			default:
				err = fmt.Errorf("%v", e)
			}
		}
	}()

	s := ""

	for len(b) > 0 {

		code, n := ReadVarintCode(b)
		b = b[n:]
		p := ProtocolWithCode(code)
		if p.Code == 0 {
			return "", fmt.Errorf("no protocol with code %d", code)
		}
		s += "/" + p.Name

		if p.Size == 0 {
			continue
		}

		size := sizeForAddr(p, b)
		a, err := addressBytesToString(p, b[:size])
		if err != nil {
			return "", err
		}
		if len(a) > 0 {
			s += "/" + a
		}
		b = b[size:]
	}

	return s, nil
}

func sizeForAddr(p Protocol, b []byte) int {
	switch {
	case p.Size > 0:
		return (p.Size / 8)
	case p.Size == 0:
		return 0
	default:
		size, n := ReadVarintCode(b)
		return size + n
	}
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
		code, n := ReadVarintCode(b)
		p := ProtocolWithCode(code)
		if p.Code == 0 {
			return [][]byte{}, fmt.Errorf("no protocol with code %d", b[0])
		}

		size := sizeForAddr(p, b[n:])
		length := n + size
		ret = append(ret, b[:length])
		b = b[length:]
	}

	return ret, nil
}

func addressStringToBytes(p Protocol, s string) ([]byte, error) {
	switch p.Code {

	case P_IP4: // ipv4
		i := net.ParseIP(s).To4()
		if i == nil {
			return nil, fmt.Errorf("failed to parse ip4 addr: %s", s)
		}
		return i, nil

	case P_IP6: // ipv6
		i := net.ParseIP(s).To16()
		if i == nil {
			return nil, fmt.Errorf("failed to parse ip6 addr: %s", s)
		}
		return i, nil

	// tcp udp dccp sctp
	case P_TCP, P_UDP, P_DCCP, P_SCTP:
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s addr: %s", p.Name, err)
		}
		if i >= 65536 {
			return nil, fmt.Errorf("failed to parse %s addr: %s", p.Name, "greater than 65536")
		}
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(i))
		return b, nil

	case P_IPFS: // ipfs
		// the address is a varint prefixed multihash string representation
		m, err := mh.FromB58String(s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ipfs addr: %s %s", s, err)
		}
		size := CodeToVarint(len(m))
		b := append(size, m...)
		return b, nil
	}

	return []byte{}, fmt.Errorf("failed to parse %s addr: unknown", p.Name)
}

func addressBytesToString(p Protocol, b []byte) (string, error) {
	switch p.Code {

	// ipv4,6
	case P_IP4, P_IP6:
		return net.IP(b).String(), nil

	// tcp udp dccp sctp
	case P_TCP, P_UDP, P_DCCP, P_SCTP:
		i := binary.BigEndian.Uint16(b)
		return strconv.Itoa(int(i)), nil

	case P_IPFS: // ipfs
		// the address is a varint-prefixed multihash string representation
		size, n := ReadVarintCode(b)
		b = b[n:]
		if len(b) != size {
			panic("inconsistent lengths")
		}
		m, err := mh.Cast(b)
		if err != nil {
			return "", err
		}
		return m.B58String(), nil
	}

	return "", fmt.Errorf("unknown protocol")
}
