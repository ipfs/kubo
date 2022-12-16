package testutils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

func SplitLines(s string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func MustOpen(name string) *os.File {
	f, err := os.Open(name)
	if err != nil {
		log.Panicf("opening %s: %s", name, err)
	}
	return f
}

// StrCat takes a bunch of strings or string slices
// and concats them all together into one string slice.
// If an arg is not one of those types, this panics.
// If an arg is an empty string, it is dropped.
func StrCat(args ...interface{}) []string {
	res := make([]string, 0)
	for _, a := range args {
		if s, ok := a.(string); ok {
			if s != "" {
				res = append(res, s)
			}
			continue
		}
		if ss, ok := a.([]string); ok {
			for _, s := range ss {
				if s != "" {
					res = append(res, s)
				}
			}
			continue
		}
		panic(fmt.Sprintf("arg '%v' must be a string or string slice, but is '%T'", a, a))
	}
	return res
}

// PreviewStr returns a preview of s, which is a prefix for logging that avoids dumping a huge string to logs.
func PreviewStr(s string) string {
	suffix := "..."
	previewLength := 10
	if len(s) < previewLength {
		previewLength = len(s)
		suffix = ""
	}
	return s[0:previewLength] + suffix
}

type JSONObj map[string]interface{}

func ToJSONStr(m JSONObj) string {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Searches for a file in a dir, then the parent dir, etc.
// If the file is not found, an empty string is returned.
func FindUp(name, dir string) string {
	curDir := dir
	for {
		entries, err := os.ReadDir(curDir)
		if err != nil {
			panic(err)
		}
		for _, e := range entries {
			if name == e.Name() {
				return filepath.Join(curDir, name)
			}
		}
		newDir := filepath.Dir(curDir)
		if newDir == curDir {
			return ""
		}
		curDir = newDir
	}
}

func RandomBytes(n int) []byte {
	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return bytes
}

// URLStrToMultiaddr converts a URL string like http://localhost:80 to a multiaddr.
func URLStrToMultiaddr(u string) multiaddr.Multiaddr {
	parsedURL, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	addrPort, err := netip.ParseAddrPort(parsedURL.Host)
	if err != nil {
		panic(err)
	}
	tcpAddr := net.TCPAddrFromAddrPort(addrPort)
	ma, err := manet.FromNetAddr(tcpAddr)
	if err != nil {
		panic(err)
	}
	return ma
}

// ForEachPar invokes f in a new goroutine for each element of s and waits for all to complete.
func ForEachPar[T any](s []T, f func(T)) {
	wg := sync.WaitGroup{}
	wg.Add(len(s))
	for _, x := range s {
		go func(x T) {
			defer wg.Done()
			f(x)
		}(x)
	}
	wg.Wait()
}
