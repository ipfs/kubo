package netmux

import (
  "net"
)

// An interface is the module connecting netmux
// to various networks (tcp, udp, webrtc, etc).
// It keeps the relevant connections open.
type Interface struct {

  // Interface network (e.g. udp4, tcp6)
  Network string

  // Own network address
  Address         string
  ResolvedAddress *net.UDPAddr

  // Connection
  conn net.Conn

  // next packets + close control channels
  Input  chan *Packet
  Output chan *Packet
  Closed chan bool
  Errors chan error
}

func NewUDPInterface(network, addr string) (*Interface, error) {
  raddr, err := net.ResolveUDPAddr(network, addr)
  if err != nil {
    return nil, err
  }

  conn, err := net.ListenUDP(network, raddr)
  if err != nil {
    return nil, err
  }

  i := &Interface{
    Network: network,
    Address: addr,
    ResolvedAddress: raddr,
    conn: conn,
  }

  go i.processUDPInput()
  go i.processOutput()
  return i, nil
}

func (i *Interface) processOutput() {
  for {
    select {
    case <-i.Closed:
      break

    case buffer := <-i.Output:
      i.conn.Write([]byte(buffer.Data))
    }
  }
}

func (i *Interface) processUDPInput() {
  for {
    select {
    case <-i.Closed:
      break

    }
  }
}

func (i *Interface) Read(buffer []byte) bool {
  _, err := i.conn.Read(buffer)
  if err != nil {
    i.Errors <- err
    i.Close()
    return false
  }
  return true
}

func (i *Interface) Close() {
  // closing net connection
  err := i.conn.Close()
  if err != nil {
    i.Errors <- err
  }

  // closing channels
  close(i.Input)
  close(i.Output)
  close(i.Closed)
  close(i.Errors)
}
