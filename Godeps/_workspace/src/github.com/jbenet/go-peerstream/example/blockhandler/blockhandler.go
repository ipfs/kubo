package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"

	ps "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream"
)

func die(err error) {
	fmt.Fprintf(os.Stderr, "error: %s\n")
	os.Exit(1)
}

func main() {
	// create a new Swarm
	swarm := ps.NewSwarm()
	defer swarm.Close()

	// tell swarm what to do with a new incoming streams.
	// EchoHandler just echos back anything they write.
	swarm.SetStreamHandler(ps.EchoHandler)

	l, err := net.Listen("tcp", "localhost:8001")
	if err != nil {
		die(err)
	}

	if _, err := swarm.AddListener(l); err != nil {
		die(err)
	}

	nc, err := net.Dial("tcp", "localhost:8001")
	if err != nil {
		die(err)
	}

	c, err := swarm.AddConn(nc)
	if err != nil {
		die(err)
	}

	nRcvStream := 0
	bio := bufio.NewReader(os.Stdin)
	swarm.SetStreamHandler(func(s *ps.Stream) {
		log("handling new stream %d", nRcvStream)
		nRcvStream++

		line, err := bio.ReadString('\n')
		if err != nil {
			die(err)
		}
		_ = line
		// line = "read: " + line
		// s.Write([]byte(line))
		s.Close()
	})

	nSndStream := 0
	for {
		<-time.After(200 * time.Millisecond)
		s, err := swarm.NewStreamWithConn(c)
		if err != nil {
			die(err)
		}
		log("sender got new stream %d", nSndStream)
		nSndStream++
		s.Wait()
	}
}

func log(s string, ifs ...interface{}) {
	fmt.Fprintf(os.Stderr, s+"\n", ifs...)
}
