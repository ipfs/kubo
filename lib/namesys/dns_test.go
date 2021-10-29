package namesys

import (
	"context"
	"fmt"
	"testing"

	opts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
)

type mockDNS struct {
	entries map[string][]string
}

func (m *mockDNS) lookupTXT(ctx context.Context, name string) (txt []string, err error) {
	txt, ok := m.entries[name]
	if !ok {
		return nil, fmt.Errorf("no TXT entry for %s", name)
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
		"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/foo/bar/baz/",
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
			"multihash.example.com.": {
				"dnslink=QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
			},
			"ipfs.example.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
			},
			"_dnslink.dipfs.example.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
			},
			"dns1.example.com.": {
				"dnslink=/ipns/ipfs.example.com",
			},
			"dns2.example.com.": {
				"dnslink=/ipns/dns1.example.com",
			},
			"multi.example.com.": {
				"some stuff",
				"dnslink=/ipns/dns1.example.com",
				"masked dnslink=/ipns/example.invalid",
			},
			"equals.example.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/=equals",
			},
			"loop1.example.com.": {
				"dnslink=/ipns/loop2.example.com",
			},
			"loop2.example.com.": {
				"dnslink=/ipns/loop1.example.com",
			},
			"_dnslink.dloop1.example.com.": {
				"dnslink=/ipns/loop2.example.com",
			},
			"_dnslink.dloop2.example.com.": {
				"dnslink=/ipns/loop1.example.com",
			},
			"bad.example.com.": {
				"dnslink=",
			},
			"withsegment.example.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/sub/segment",
			},
			"withrecsegment.example.com.": {
				"dnslink=/ipns/withsegment.example.com/subsub",
			},
			"withtrailing.example.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/sub/",
			},
			"withtrailingrec.example.com.": {
				"dnslink=/ipns/withtrailing.example.com/segment/",
			},
			"double.example.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
			},
			"_dnslink.double.example.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
			},
			"double.conflict.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD",
			},
			"_dnslink.conflict.example.com.": {
				"dnslink=/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjE",
			},
			"fqdn.example.com.": {
				"dnslink=/ipfs/QmYvMB9yrsSf7RKBghkfwmHJkzJhW2ZgVwq3LxBXXPasFr",
			},
			"en.wikipedia-on-ipfs.org.": {
				"dnslink=/ipfs/bafybeiaysi4s6lnjev27ln5icwm6tueaw2vdykrtjkwiphwekaywqhcjze",
			},
			"custom.non-icann.tldextravaganza.": {
				"dnslink=/ipfs/bafybeieto6mcuvqlechv4iadoqvnffondeiwxc2bcfcewhvpsd2odvbmvm",
			},
			"singlednslabelshouldbeok.": {
				"dnslink=/ipfs/bafybeih4a6ylafdki6ailjrdvmr7o4fbbeceeeuty4v3qyyouiz5koqlpi",
			},
			"www.wealdtech.eth.": {
				"dnslink=/ipns/ipfs.example.com",
			},
		},
	}
}

func TestDNSResolution(t *testing.T) {
	mock := newMockDNS()
	r := &DNSResolver{lookupTXT: mock.lookupTXT}
	testResolution(t, r, "multihash.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "ipfs.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "dipfs.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "dns1.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "dns1.example.com", 1, "/ipns/ipfs.example.com", ErrResolveRecursion)
	testResolution(t, r, "dns2.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "dns2.example.com", 1, "/ipns/dns1.example.com", ErrResolveRecursion)
	testResolution(t, r, "dns2.example.com", 2, "/ipns/ipfs.example.com", ErrResolveRecursion)
	testResolution(t, r, "multi.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "multi.example.com", 1, "/ipns/dns1.example.com", ErrResolveRecursion)
	testResolution(t, r, "multi.example.com", 2, "/ipns/ipfs.example.com", ErrResolveRecursion)
	testResolution(t, r, "equals.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/=equals", nil)
	testResolution(t, r, "loop1.example.com", 1, "/ipns/loop2.example.com", ErrResolveRecursion)
	testResolution(t, r, "loop1.example.com", 2, "/ipns/loop1.example.com", ErrResolveRecursion)
	testResolution(t, r, "loop1.example.com", 3, "/ipns/loop2.example.com", ErrResolveRecursion)
	testResolution(t, r, "loop1.example.com", opts.DefaultDepthLimit, "/ipns/loop1.example.com", ErrResolveRecursion)
	testResolution(t, r, "dloop1.example.com", 1, "/ipns/loop2.example.com", ErrResolveRecursion)
	testResolution(t, r, "dloop1.example.com", 2, "/ipns/loop1.example.com", ErrResolveRecursion)
	testResolution(t, r, "dloop1.example.com", 3, "/ipns/loop2.example.com", ErrResolveRecursion)
	testResolution(t, r, "dloop1.example.com", opts.DefaultDepthLimit, "/ipns/loop1.example.com", ErrResolveRecursion)
	testResolution(t, r, "bad.example.com", opts.DefaultDepthLimit, "", ErrResolveFailed)
	testResolution(t, r, "withsegment.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/sub/segment", nil)
	testResolution(t, r, "withrecsegment.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/sub/segment/subsub", nil)
	testResolution(t, r, "withsegment.example.com/test1", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/sub/segment/test1", nil)
	testResolution(t, r, "withrecsegment.example.com/test2", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/sub/segment/subsub/test2", nil)
	testResolution(t, r, "withrecsegment.example.com/test3/", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/sub/segment/subsub/test3/", nil)
	testResolution(t, r, "withtrailingrec.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD/sub/segment/", nil)
	testResolution(t, r, "double.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "conflict.example.com", opts.DefaultDepthLimit, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjE", nil)
	testResolution(t, r, "fqdn.example.com.", opts.DefaultDepthLimit, "/ipfs/QmYvMB9yrsSf7RKBghkfwmHJkzJhW2ZgVwq3LxBXXPasFr", nil)
	testResolution(t, r, "en.wikipedia-on-ipfs.org", 2, "/ipfs/bafybeiaysi4s6lnjev27ln5icwm6tueaw2vdykrtjkwiphwekaywqhcjze", nil)
	testResolution(t, r, "custom.non-icann.tldextravaganza.", 2, "/ipfs/bafybeieto6mcuvqlechv4iadoqvnffondeiwxc2bcfcewhvpsd2odvbmvm", nil)
	testResolution(t, r, "singlednslabelshouldbeok", 2, "/ipfs/bafybeih4a6ylafdki6ailjrdvmr7o4fbbeceeeuty4v3qyyouiz5koqlpi", nil)
	testResolution(t, r, "www.wealdtech.eth", 1, "/ipns/ipfs.example.com", ErrResolveRecursion)
	testResolution(t, r, "www.wealdtech.eth", 2, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
	testResolution(t, r, "www.wealdtech.eth", 2, "/ipfs/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", nil)
}
