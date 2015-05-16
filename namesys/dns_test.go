package namesys

import (
	"fmt"
	"testing"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

type mockDNS struct {
	entries map[string][]string
}

func (m *mockDNS) lookupTXT(name string) (txt []string, err error) {
	txt, ok := m.entries[name]
	if !ok {
		return nil, fmt.Errorf("No TXT entry for %s", name)
	}
	return txt, nil
}

func TestDnsEntryParsing(t *testing.T) {
	goodEntries := []string{
		"QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
		"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
		"dnslink=/ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
		"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/foo",
		"dnslink=/ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/bar",
		"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/foo/bar/baz",
		"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
	}

	badEntries := []string{
		"QmYhE8xgFCjGcz6PHgnvJz5NOTCORRECT",
		"quux=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
		"dnslink=",
		"dnslink=/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/foo",
		"dnslink=ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/bar",
	}

	for _, e := range goodEntries {
		_, err := parseEntry(e)
		if err != nil {
			t.Log("expected entry to parse correctly!")
			t.Log(e)
			t.Fatal(err)
		}
	}

	for _, e := range badEntries {
		_, err := parseEntry(e)
		if err == nil {
			t.Log("expected entry parse to fail!")
			t.Fatal(err)
		}
	}
}

func newMockDNS() *mockDNS {
	return &mockDNS{
		entries: map[string][]string{
			"multihash.example.com": []string{
				"dnslink=QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
			},
			"ipfs.example.com": []string{
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
			},
			"dns1.example.com": []string{
				"dnslink=/ipns/ipfs.example.com",
			},
			"dns2.example.com": []string{
				"dnslink=/ipns/dns1.example.com",
			},
			"multi.example.com": []string{
				"some stuff",
				"dnslink=/ipns/dns1.example.com",
				"masked dnslink=/ipns/example.invalid",
			},
			"equals.example.com": []string{
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/=equals",
			},
			"loop1.example.com": []string{
				"dnslink=/ipns/loop2.example.com",
			},
			"loop2.example.com": []string{
				"dnslink=/ipns/loop1.example.com",
			},
			"bad.example.com": []string{
				"dnslink=",
			},
		},
	}
}

func testResolution(t *testing.T, resolver Resolver, name string, depth int, expected string, expError error) {
	p, err := resolver.ResolveN(context.Background(), name, depth)
	if err != expError {
		t.Fatal(fmt.Errorf(
			"Expected %s with a depth of %d to have a '%s' error, but got '%s'",
			name, depth, expError, err))
	}
	if p.String() != expected {
		t.Fatal(fmt.Errorf(
			"%s with depth %d resolved to %s != %s",
			name, depth, p.String(), expected))
	}
}

func TestDNSResolution(t *testing.T) {
	mock := newMockDNS()
	r := &DNSResolver{lookupTXT: mock.lookupTXT}
	testResolution(t, r, "multihash.example.com", DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "ipfs.example.com", DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "dns1.example.com", DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "dns1.example.com", 1, "/ipns/ipfs.example.com", ErrResolveRecursion)
	testResolution(t, r, "dns2.example.com", DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "dns2.example.com", 1, "/ipns/dns1.example.com", ErrResolveRecursion)
	testResolution(t, r, "dns2.example.com", 2, "/ipns/ipfs.example.com", ErrResolveRecursion)
	testResolution(t, r, "multi.example.com", DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "multi.example.com", 1, "/ipns/dns1.example.com", ErrResolveRecursion)
	testResolution(t, r, "multi.example.com", 2, "/ipns/ipfs.example.com", ErrResolveRecursion)
	testResolution(t, r, "equals.example.com", DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/=equals", nil)
	testResolution(t, r, "loop1.example.com", 1, "/ipns/loop2.example.com", ErrResolveRecursion)
	testResolution(t, r, "loop1.example.com", 2, "/ipns/loop1.example.com", ErrResolveRecursion)
	testResolution(t, r, "loop1.example.com", 3, "/ipns/loop2.example.com", ErrResolveRecursion)
	testResolution(t, r, "loop1.example.com", DefaultDepthLimit, "/ipns/loop1.example.com", ErrResolveRecursion)
	testResolution(t, r, "bad.example.com", DefaultDepthLimit, "", ErrResolveFailed)
}
