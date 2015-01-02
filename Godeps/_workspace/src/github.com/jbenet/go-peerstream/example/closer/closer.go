package main

import (
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

	hello := []byte("hello")
	goodbye := []byte("goodbye")
	swarm.SetStreamHandler(func(s *ps.Stream) {
		go func() {
			log("handler: got new stream.")
			// s.Wait()
			// log("handler: done waiting on new stream.")
			buf := make([]byte, len(hello))
			s.Read(buf)
			log("handler: read: %s", buf)
			s.Write(goodbye)
			log("handler: wrote: %s", goodbye)
			s.Close()
			log("handler: closed.")
		}()
	})

	for {
		s, err := swarm.NewStreamWithConn(c)
		if err != nil {
			die(err)
		}
		// s.Wait()
		log("sender: got new stream")
		for {
			<-time.After(500 * time.Millisecond)
			log("sender: writing hello...")
			if _, err := s.Write(hello); err != nil {
				log("sender: write error: %s", err)
				break
			}
			buf := make([]byte, len(goodbye))
			if _, err := s.Read(buf); err != nil {
				log("sender: read error: %s", err)
				break
			}
		}
		if err := s.Close(); err != nil {
			log("sender: close error: %s", err)
		}
	}
}

func log(s string, ifs ...interface{}) {
	fmt.Fprintf(os.Stderr, s+"\n", ifs...)
}
