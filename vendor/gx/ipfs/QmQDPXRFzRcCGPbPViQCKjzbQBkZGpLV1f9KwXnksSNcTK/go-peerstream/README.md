# go-peerstream p2p multi-multixplexing

Package peerstream is a peer-to-peer networking library that multiplexes
connections to many hosts. It tried to simplify the complexity of:

* accepting incoming connections over **multiple** listeners
* dialing outgoing connections over **multiple** transports
* multiplexing **multiple** connections per-peer
* multiplexing **multiple** different servers or protocols
* handling backpressure correctly
* handling stream multiplexing (we use SPDY, but maybe QUIC some day)
* providing a **simple** interface to the user

### Godoc: https://godoc.org/github.com/jbenet/go-peerstream

---

See this working [example/example.go](example/example):

```Go
package main

import (
  "fmt"
  "io"
  "net"
  "os"

  ps "github.com/jbenet/go-peerstream"
)

func main() {
  // create a new Swarm
  swarm := ps.NewSwarm()
  defer swarm.Close()

  // tell swarm what to do with a new incoming streams.
  // EchoHandler just echos back anything they write.
  swarm.SetStreamHandler(ps.EchoHandler)

  // Okay, let's try listening on some transports
  l1, err := net.Listen("tcp", "localhost:8001")
  if err != nil {
    panic(err)
  }

  l2, err := net.Listen("tcp", "localhost:8002")
  if err != nil {
    panic(err)
  }

  // tell swarm to accept incoming connections on these
  // listeners. Swarm will start accepting new connections.
  if err := swarm.AddListener(l1); err != nil {
    panic(err)
  }
  if err := swarm.AddListener(l2); err != nil {
    panic(err)
  }

  // ok, let's try some outgoing connections
  nc1, err := net.Dial("tcp", "localhost:8001")
  if err != nil {
    panic(err)
  }

  nc2, err := net.Dial("tcp", "localhost:8002")
  if err != nil {
    panic(err)
  }

  // add them to the swarm
  c1, err := swarm.AddConn(nc1)
  if err != nil {
    panic(err)
  }
  c2, err := swarm.AddConn(nc2)
  if err != nil {
    panic(err)
  }

  // Swarm treats listeners as sources of new connections and does
  // not distinguish between outgoing or incoming connections.
  // It provides the net.Conn to the StreamHandler so you can
  // distinguish between them however you wish.

  // now let's try opening some streams!
  // You can specify what connection you want to use
  s1, err := swarm.NewStreamWithConn(c1)
  if err != nil {
    panic(err)
  }

  // Or, you can specify a SelectConn function that picks between all
  // (it calls NewStreamWithConn underneath the hood)
  s2, err := swarm.NewStreamSelectConn(func(conns []*ps.Conn) *ps.Conn {
    if len(conns) > 0 {
      return conns[0]
    }
    return nil
  })
  if err != nil {
    panic(err)
  }

  // Or, you can bind connections to ConnGroup ids. You can bind a conn to
  // multiple groups. And, if conn wasn't in swarm, it calls swarm.AddConn.
  // You can use any Go `KeyType` as a group A `KeyType` as in maps...)
  swarm.AddConnToGroup(c2, 1)

  // And then use that group to select a connection. Swarm will use any
  // connection it finds in that group, using a SelectConn you can rebind:
  //   swarm.SetGroupSelectConn(1, SelectConn)
  //   swarm.SetDegaultGroupSelectConn(SelectConn)
  s3, err := swarm.NewStreamWithGroup(1)
  if err != nil {
    panic(err)
  }

  // Why groups? It's because with many connections, and many transports,
  // and many Servers (or Protocols), we can use the Swarm to associate
  // a different StreamHandlers per group, and to let us create NewStreams
  // on a given group.

  // Ok, we have streams. now what. Use them! Our Streams are basically
  // streams from github.com/docker/spdystream, so they work the same
  // way:

  for i, stream := range []ps.Stream{s1, s2, s3} {
    stream.Wait()
    str := "stream %d ready:"
    fmt.Fprintf(stream, str, i)

    buf := make([]byte, len(str))
    stream.Read(buf)
    fmt.Println(string(buf))
  }

  go io.Copy(os.Stdout, s1)
  go io.Copy(os.Stdout, s2)
  go io.Copy(os.Stdout, s3)
  io.Copy(io.MultiWriter(s1, s2, s3), os.Stdin)
}

func log(s string) {
  fmt.Fprintf(os.Stderr, s+"\n")
}
```
