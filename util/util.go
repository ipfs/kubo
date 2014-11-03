package util

import (
	"errors"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/mitchellh/go-homedir"
)

// Debug is a global flag for debugging.
var Debug bool

// ErrNotImplemented signifies a function has not been implemented yet.
var ErrNotImplemented = errors.New("Error: not implemented yet.")

// ErrTimeout implies that a timeout has been triggered
var ErrTimeout = errors.New("Error: Call timed out.")

// ErrSeErrSearchIncomplete implies that a search type operation didnt
// find the expected node, but did find 'a' node.
var ErrSearchIncomplete = errors.New("Error: Search Incomplete.")

// ErrNotFound is returned when a search fails to find anything
var ErrNotFound = ds.ErrNotFound

// ErrNoSuchLogger is returned when the util pkg is asked for a non existant logger
var ErrNoSuchLogger = errors.New("Error: No such logger")

// TildeExpansion expands a filename, which may begin with a tilde.
func TildeExpansion(filename string) (string, error) {
	return homedir.Expand(filename)
}

// ExpandPathnames takes a set of paths and turns them into absolute paths
func ExpandPathnames(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		abspath, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		out = append(out, abspath)
	}
	return out, nil
}

// byteChanReader wraps a byte chan in a reader
type byteChanReader struct {
	in  chan []byte
	buf []byte
}

func NewByteChanReader(in chan []byte) io.Reader {
	return &byteChanReader{in: in}
}

func (bcr *byteChanReader) Read(output []byte) (int, error) {
	remain := output
	remainLen := len(output)
	outputLen := 0
	more := false
	next := bcr.buf

	for {
		n := copy(remain, next)
		remainLen -= n
		outputLen += n
		if remainLen == 0 {
			bcr.buf = next[n:]
			return outputLen, nil
		}

		remain = remain[n:]
		next, more = <-bcr.in
		if !more {
			return outputLen, io.EOF
		}
	}
}

type randGen struct {
	rand.Rand
}

func NewTimeSeededRand() io.Reader {
	src := rand.NewSource(time.Now().UnixNano())
	return &randGen{
		Rand: *rand.New(src),
	}
}

func (r *randGen) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = byte(r.Rand.Intn(255))
	}
	return len(p), nil
}

// GetenvBool is the way to check an env var as a boolean
func GetenvBool(name string) bool {
	v := strings.ToLower(os.Getenv(name))
	return v == "true" || v == "t" || v == "1"
}

// IsLoopbackAddr returns whether or not the ip portion of the passed in multiaddr
// string is a loopback address
func IsLoopbackAddr(addr string) bool {
	loops := []string{"/ip4/127.0.0.1", "/ip6/::1"}
	for _, loop := range loops {
		if strings.HasPrefix(addr, loop) {
			return true
		}
	}
	return false
}

// GetLocalAddresses returns a list of ip addresses associated with
// the local machine
func GetLocalAddresses() ([]ma.Multiaddr, error) {
	// Enumerate interfaces on this machine
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var maddrs []ma.Multiaddr
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			log.Warningf("Skipping addr: %s", err)
			continue
		}
		// Check each address and convert to a multiaddr
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:

				// Build multiaddr
				maddr, err := manet.FromIP(v.IP)
				if err != nil {
					log.Errorf("maddr parsing error: %s", err)
					continue
				}

				// Dont list loopback addresses
				if IsLoopbackAddr(maddr.String()) {
					continue
				}
				maddrs = append(maddrs, maddr)
			default:
				// Not sure if any other types will show up here
				log.Errorf("Got '%s' type = '%s'", v, reflect.TypeOf(v))
			}
		}
	}
	return maddrs, nil
}
