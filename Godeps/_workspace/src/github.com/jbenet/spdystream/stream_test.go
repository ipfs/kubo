package spdystream

import (
	"net"
	"net/http"
	"sync"
	"testing"
)

func TestStreamReset(t *testing.T) {
	var wg sync.WaitGroup
	listen := "localhost:7743"
	server, serverErr := runServer(listen, &wg)
	if serverErr != nil {
		t.Fatalf("Error initializing server: %s", serverErr)
	}

	conn, dialErr := net.Dial("tcp", listen)
	if dialErr != nil {
		t.Fatalf("Error dialing server: %s", dialErr)
	}

	spdyConn, spdyErr := NewConnection(conn, false)
	if spdyErr != nil {
		t.Fatalf("Error creating spdy connection: %s", spdyErr)
	}
	go spdyConn.Serve(NoOpStreamHandler)

	authenticated = true
	stream, streamErr := spdyConn.CreateStream(http.Header{}, nil, false)
	if streamErr != nil {
		t.Fatalf("Error creating stream: %s", streamErr)
	}

	buf := []byte("dskjahfkdusahfkdsahfkdsafdkas")
	for i := 0; i < 10; i++ {
		if _, err := stream.Write(buf); err != nil {
			t.Fatalf("Error writing to stream: %s", err)
		}
	}
	for i := 0; i < 10; i++ {
		if _, err := stream.Read(buf); err != nil {
			t.Fatalf("Error reading from stream: %s", err)
		}
	}

	// fmt.Printf("Resetting...\n")
	if err := stream.Reset(); err != nil {
		t.Fatalf("Error reseting stream: %s", err)
	}

	closeErr := server.Close()
	if closeErr != nil {
		t.Fatalf("Error shutting down server: %s", closeErr)
	}
	wg.Wait()
}

func TestStreamResetWithDataRemaining(t *testing.T) {
	var wg sync.WaitGroup
	listen := "localhost:7743"
	server, serverErr := runServer(listen, &wg)
	if serverErr != nil {
		t.Fatalf("Error initializing server: %s", serverErr)
	}

	conn, dialErr := net.Dial("tcp", listen)
	if dialErr != nil {
		t.Fatalf("Error dialing server: %s", dialErr)
	}

	spdyConn, spdyErr := NewConnection(conn, false)
	if spdyErr != nil {
		t.Fatalf("Error creating spdy connection: %s", spdyErr)
	}
	go spdyConn.Serve(NoOpStreamHandler)

	authenticated = true
	stream, streamErr := spdyConn.CreateStream(http.Header{}, nil, false)
	if streamErr != nil {
		t.Fatalf("Error creating stream: %s", streamErr)
	}

	buf := []byte("dskjahfkdusahfkdsahfkdsafdkas")
	for i := 0; i < 10; i++ {
		if _, err := stream.Write(buf); err != nil {
			t.Fatalf("Error writing to stream: %s", err)
		}
	}

	// read a bit to make sure a goroutine gets to <-dataChan
	if _, err := stream.Read(buf); err != nil {
		t.Fatalf("Error reading from stream: %s", err)
	}

	// fmt.Printf("Resetting...\n")
	if err := stream.Reset(); err != nil {
		t.Fatalf("Error reseting stream: %s", err)
	}

	closeErr := server.Close()
	if closeErr != nil {
		t.Fatalf("Error shutting down server: %s", closeErr)
	}
	wg.Wait()
}
