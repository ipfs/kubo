package netmux

// The netmux module provides a "network multiplexer".
// The core idea is to have the client be able to connect to
// many different networks (potentially over different transports)
// and multiplex everything over one interface.

type Netmux struct {
  // the list of NetMux interfaces
  Interfaces []*Interface

  // The channels to send/recv from
  Incoming <-chan *Packet
  Outgoing chan<- *Packet

  // internally managed other side of channels
  incomingSrc chan<- *Packet
  outgoingSrc <-chan *Packet
}

// Warning: will probably change to adopt multiaddr format
type Packet struct {
  // the network addresses to send to
  // e.g. tcp4://127.0.0.1:12345
  NetAddrTo string

  // the network addresses to recv from
  // e.g. tcp4://127.0.0.1:12345
  // may be left blank to select one automatically.
  NetAddrFrom string

  // the data to send.
  Data []byte
}

func NewNetmux() *Netmux {
  n := &Netmux{}

  // setup channels
  och := make(chan *Packet)
  ich := make(chan *Packet)
  n.Incoming, n.incomingSrc = ich, ich
  n.Outgoing, n.outgoingSrc = och, och

  return n
}
