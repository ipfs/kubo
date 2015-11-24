package multiaddr

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Protocol is a Multiaddr protocol description structure.
type Protocol struct {
	Code  int
	Size  int // a size of -1 indicates a length-prefixed variable size
	Name  string
	VCode []byte
}

// replicating table here to:
// 1. avoid parsing the csv
// 2. ensuring errors in the csv don't screw up code.
// 3. changing a number has to happen in two places.
const (
	P_IP4   = 4
	P_TCP   = 6
	P_UDP   = 17
	P_DCCP  = 33
	P_IP6   = 41
	P_SCTP  = 132
	P_UTP   = 301
	P_UDT   = 302
	P_IPFS  = 421
	P_HTTP  = 480
	P_HTTPS = 443
)

// These are special sizes
const (
	LengthPrefixedVarSize = -1
)

// Protocols is the list of multiaddr protocols supported by this module.
var Protocols = []Protocol{
	Protocol{P_IP4, 32, "ip4", CodeToVarint(P_IP4)},
	Protocol{P_TCP, 16, "tcp", CodeToVarint(P_TCP)},
	Protocol{P_UDP, 16, "udp", CodeToVarint(P_UDP)},
	Protocol{P_DCCP, 16, "dccp", CodeToVarint(P_DCCP)},
	Protocol{P_IP6, 128, "ip6", CodeToVarint(P_IP6)},
	// these require varint:
	Protocol{P_SCTP, 16, "sctp", CodeToVarint(P_SCTP)},
	Protocol{P_UTP, 0, "utp", CodeToVarint(P_UTP)},
	Protocol{P_UDT, 0, "udt", CodeToVarint(P_UDT)},
	Protocol{P_HTTP, 0, "http", CodeToVarint(P_HTTP)},
	Protocol{P_HTTPS, 0, "https", CodeToVarint(P_HTTPS)},
	Protocol{P_IPFS, LengthPrefixedVarSize, "ipfs", CodeToVarint(P_IPFS)},
}

// ProtocolWithName returns the Protocol description with given string name.
func ProtocolWithName(s string) Protocol {
	for _, p := range Protocols {
		if p.Name == s {
			return p
		}
	}
	return Protocol{}
}

// ProtocolWithCode returns the Protocol description with given protocol code.
func ProtocolWithCode(c int) Protocol {
	for _, p := range Protocols {
		if p.Code == c {
			return p
		}
	}
	return Protocol{}
}

// ProtocolsWithString returns a slice of protocols matching given string.
func ProtocolsWithString(s string) ([]Protocol, error) {
	s = strings.Trim(s, "/")
	sp := strings.Split(s, "/")
	if len(sp) == 0 {
		return nil, nil
	}

	t := make([]Protocol, len(sp))
	for i, name := range sp {
		p := ProtocolWithName(name)
		if p.Code == 0 {
			return nil, fmt.Errorf("no protocol with name: %s", name)
		}
		t[i] = p
	}
	return t, nil
}

// CodeToVarint converts an integer to a varint-encoded []byte
func CodeToVarint(num int) []byte {
	buf := make([]byte, (num/7)+1) // varint package is uint64
	n := binary.PutUvarint(buf, uint64(num))
	return buf[:n]
}

// VarintToCode converts a varint-encoded []byte to an integer protocol code
func VarintToCode(buf []byte) int {
	num, _ := ReadVarintCode(buf)
	return num
}

// ReadVarintCode reads a varint code from the beginning of buf.
// returns the code, and the number of bytes read.
func ReadVarintCode(buf []byte) (int, int) {
	num, n := binary.Uvarint(buf)
	if n < 0 {
		panic("varints larger than uint64 not yet supported")
	}
	return int(num), n
}
