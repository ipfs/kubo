package reuseport

import (
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"testing"
)

func echo(c net.Conn) {
	io.Copy(c, c)
	c.Close()
}

func packetEcho(c net.PacketConn) {
	buf := make([]byte, 65536)
	for {
		n, addr, err := c.ReadFrom(buf)
		if err != nil {
			return
		}
		if _, err := c.WriteTo(buf[:n], addr); err != nil {
			return
		}
	}
}

func acceptAndEcho(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		go echo(c)
	}
}

func CI() bool {
	return os.Getenv("TRAVIS") == "true"
}

func TestStreamListenSamePort(t *testing.T) {

	// any ports
	any := [][]string{
		[]string{"tcp", "0.0.0.0:0"},
		[]string{"tcp4", "0.0.0.0:0"},
		[]string{"tcp6", "[::]:0"},

		[]string{"tcp", "127.0.0.1:0"},
		[]string{"tcp", "[::1]:0"},
		[]string{"tcp4", "127.0.0.1:0"},
		[]string{"tcp6", "[::1]:0"},
	}

	// specific ports. off in CI
	specific := [][]string{
		[]string{"tcp", "127.0.0.1:5556"},
		[]string{"tcp", "[::1]:5557"},
		[]string{"tcp4", "127.0.0.1:5558"},
		[]string{"tcp6", "[::1]:5559"},
	}

	testCases := any
	if !CI() {
		testCases = append(testCases, specific...)
	}

	for _, tcase := range testCases {
		network := tcase[0]
		addr := tcase[1]
		t.Log("testing", network, addr)

		l1, err := Listen(network, addr)
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l1.Close()
		t.Log("listening", l1.Addr())

		l2, err := Listen(l1.Addr().Network(), l1.Addr().String())
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l2.Close()
		t.Log("listening", l2.Addr())

		l3, err := Listen(l2.Addr().Network(), l2.Addr().String())
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l3.Close()
		t.Log("listening", l3.Addr())

		if l1.Addr().String() != l2.Addr().String() {
			t.Fatal("addrs should match", l1.Addr(), l2.Addr())
		}

		if l1.Addr().String() != l3.Addr().String() {
			t.Fatal("addrs should match", l1.Addr(), l3.Addr())
		}
	}
}

func TestPacketListenSamePort(t *testing.T) {

	// any ports
	any := [][]string{
		[]string{"udp", "0.0.0.0:0"},
		[]string{"udp4", "0.0.0.0:0"},
		[]string{"udp6", "[::]:0"},

		[]string{"udp", "127.0.0.1:0"},
		[]string{"udp", "[::1]:0"},
		[]string{"udp4", "127.0.0.1:0"},
		[]string{"udp6", "[::1]:0"},
	}

	// specific ports. off in CI
	specific := [][]string{
		[]string{"udp", "127.0.0.1:5560"},
		[]string{"udp", "[::1]:5561"},
		[]string{"udp4", "127.0.0.1:5562"},
		[]string{"udp6", "[::1]:5563"},
	}

	testCases := any
	if !CI() {
		testCases = append(testCases, specific...)
	}

	for _, tcase := range testCases {
		network := tcase[0]
		addr := tcase[1]
		t.Log("testing", network, addr)

		l1, err := ListenPacket(network, addr)
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l1.Close()
		t.Log("listening", l1.LocalAddr())

		l2, err := ListenPacket(l1.LocalAddr().Network(), l1.LocalAddr().String())
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l2.Close()
		t.Log("listening", l2.LocalAddr())

		l3, err := ListenPacket(l2.LocalAddr().Network(), l2.LocalAddr().String())
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l3.Close()
		t.Log("listening", l3.LocalAddr())

		if l1.LocalAddr().String() != l2.LocalAddr().String() {
			t.Fatal("addrs should match", l1.LocalAddr(), l2.LocalAddr())
		}

		if l1.LocalAddr().String() != l3.LocalAddr().String() {
			t.Fatal("addrs should match", l1.LocalAddr(), l3.LocalAddr())
		}
	}
}

func TestStreamListenDialSamePort(t *testing.T) {

	any := [][]string{
		[]string{"tcp", "0.0.0.0:0", "0.0.0.0:0"},
		[]string{"tcp4", "0.0.0.0:0", "0.0.0.0:0"},
		[]string{"tcp6", "[::]:0", "[::]:0"},

		[]string{"tcp", "127.0.0.1:0", "127.0.0.1:0"},
		[]string{"tcp4", "127.0.0.1:0", "127.0.0.1:0"},
		[]string{"tcp6", "[::1]:0", "[::1]:0"},
	}

	specific := [][]string{
		[]string{"tcp", "127.0.0.1:0", "127.0.0.1:5571"},
		[]string{"tcp4", "127.0.0.1:0", "127.0.0.1:5573"},
		[]string{"tcp6", "[::1]:0", "[::1]:5574"},
		[]string{"tcp", "127.0.0.1:5570", "127.0.0.1:0"},
		[]string{"tcp4", "127.0.0.1:5572", "127.0.0.1:0"},
		[]string{"tcp6", "[::1]:5573", "[::1]:0"},
	}

	testCases := any
	if !CI() {
		testCases = append(testCases, specific...)
	}

	for _, tcase := range testCases {
		t.Log("testing", tcase)
		network := tcase[0]
		addr1 := tcase[1]
		addr2 := tcase[2]

		l1, err := Listen(network, addr1)
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l1.Close()
		t.Log("listening", l1.Addr())

		l2, err := Listen(network, addr2)
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l2.Close()
		t.Log("listening", l2.Addr())

		go acceptAndEcho(l1)
		go acceptAndEcho(l2)

		c1, err := Dial(network, l1.Addr().String(), l2.Addr().String())
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer c1.Close()
		t.Log("dialed", c1, c1.LocalAddr(), c1.RemoteAddr())

		if getPort(l1.Addr()) != getPort(c1.LocalAddr()) {
			t.Fatal("addrs should match", l1.Addr(), c1.LocalAddr())
		}

		if getPort(l2.Addr()) != getPort(c1.RemoteAddr()) {
			t.Fatal("addrs should match", l2.Addr(), c1.RemoteAddr())
		}

		hello1 := []byte("hello world")
		hello2 := make([]byte, len(hello1))
		if _, err := c1.Write(hello1); err != nil {
			t.Fatal(err)
			continue
		}

		if _, err := c1.Read(hello2); err != nil {
			t.Fatal(err)
			continue
		}

		if !bytes.Equal(hello1, hello2) {
			t.Fatal("echo failed", string(hello1), "!=", string(hello2))
		}
		t.Log("echoed", string(hello2))
		c1.Close()
	}
}

func TestPacketListenDialSamePort(t *testing.T) {

	any := [][]string{
		[]string{"udp", "0.0.0.0:0", "0.0.0.0:0"},
		[]string{"udp4", "0.0.0.0:0", "0.0.0.0:0"},
		[]string{"udp6", "[::]:0", "[::]:0"},

		[]string{"udp", "127.0.0.1:0", "127.0.0.1:0"},
		[]string{"udp4", "127.0.0.1:0", "127.0.0.1:0"},
		[]string{"udp6", "[::1]:0", "[::1]:0"},
	}

	specific := [][]string{
		[]string{"udp", "127.0.0.1:5670", "127.0.0.1:5671"},
		[]string{"udp4", "127.0.0.1:5672", "127.0.0.1:5673"},
		[]string{"udp6", "[::1]:5673", "[::1]:5674"},
	}

	testCases := any
	if !CI() {
		testCases = append(testCases, specific...)
	}

	for _, tcase := range testCases {
		t.Log("testing", tcase)
		network := tcase[0]
		addr1 := tcase[1]
		addr2 := tcase[2]

		l1, err := ListenPacket(network, addr1)
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l1.Close()
		t.Log("listening", l1.LocalAddr())

		l2, err := ListenPacket(network, addr2)
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer l2.Close()
		t.Log("listening", l2.LocalAddr())

		go packetEcho(l1)
		go packetEcho(l2)

		c1, err := Dial(network, l1.LocalAddr().String(), l2.LocalAddr().String())
		if err != nil {
			t.Fatal(err)
			continue
		}
		defer c1.Close()
		t.Log("dialed", c1.LocalAddr(), c1.RemoteAddr())

		if getPort(l1.LocalAddr()) != getPort(c1.LocalAddr()) {
			t.Fatal("addrs should match", l1.LocalAddr(), c1.LocalAddr())
		}

		if getPort(l2.LocalAddr()) != getPort(c1.RemoteAddr()) {
			t.Fatal("addrs should match", l2.LocalAddr(), c1.RemoteAddr())
		}

		hello1 := []byte("hello world")
		hello2 := make([]byte, len(hello1))
		if _, err := c1.Write(hello1); err != nil {
			t.Fatal(err)
			continue
		}

		if _, err := c1.Read(hello2); err != nil {
			t.Fatal(err)
			continue
		}

		if !bytes.Equal(hello1, hello2) {
			t.Fatal("echo failed", string(hello1), "!=", string(hello2))
		}
		t.Log("echoed", string(hello2))
	}
}

func TestUnixNotSupported(t *testing.T) {

	testCases := [][]string{
		[]string{"unix", "/tmp/foo"},
	}

	for _, tcase := range testCases {
		network := tcase[0]
		addr := tcase[1]
		t.Log("testing", network, addr)

		_, err := Listen(network, addr)
		if err == nil {
			t.Fatal("unix supported")
			continue
		}
	}
}

func getPort(a net.Addr) string {
	if a == nil {
		return ""
	}
	s := strings.Split(a.String(), ":")
	if len(s) > 1 {
		return s[1]
	}
	return ""
}
