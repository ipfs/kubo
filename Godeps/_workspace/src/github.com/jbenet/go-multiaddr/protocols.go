package multiaddr

// Protocol is a Multiaddr protocol description structure.
type Protocol struct {
	Code int
	Size int
	Name string
}

// replicating table here to:
// 1. avoid parsing the csv
// 2. ensuring errors in the csv don't screw up code.
// 3. changing a number has to happen in two places.
const (
	P_IP4  = 4
	P_TCP  = 6
	P_UDP  = 17
	P_DCCP = 33
	P_IP6  = 41
	P_SCTP = 132
)

// Protocols is the list of multiaddr protocols supported by this module.
var Protocols = []*Protocol{
	&Protocol{P_IP4, 32, "ip4"},
	&Protocol{P_TCP, 16, "tcp"},
	&Protocol{P_UDP, 16, "udp"},
	&Protocol{P_DCCP, 16, "dccp"},
	&Protocol{P_IP6, 128, "ip6"},
	// these require varint:
	&Protocol{P_SCTP, 16, "sctp"},
	// {480, 0, "http"},
	// {443, 0, "https"},
}

// ProtocolWithName returns the Protocol description with given string name.
func ProtocolWithName(s string) *Protocol {
	for _, p := range Protocols {
		if p.Name == s {
			return p
		}
	}
	return nil
}

// ProtocolWithCode returns the Protocol description with given protocol code.
func ProtocolWithCode(c int) *Protocol {
	for _, p := range Protocols {
		if p.Code == c {
			return p
		}
	}
	return nil
}
