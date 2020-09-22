package corehttp

import (
	"errors"
	"net/http/httptest"
	"testing"

	cid "github.com/ipfs/go-cid"
	config "github.com/ipfs/go-ipfs-config"
)

func TestToSubdomainURL(t *testing.T) {
	r := httptest.NewRequest("GET", "http://request-stub.example.com", nil)
	for _, test := range []struct {
		// in:
		hostname string
		path     string
		// out:
		url string
		err error
	}{
		// DNSLink
		{"localhost", "/ipns/dnslink.io", "http://dnslink.io.ipns.localhost/", nil},
		// Hostname with port
		{"localhost:8080", "/ipns/dnslink.io", "http://dnslink.io.ipns.localhost:8080/", nil},
		// CIDv0 → CIDv1base32
		{"localhost", "/ipfs/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", "http://bafybeif7a7gdklt6hodwdrmwmxnhksctcuav6lfxlcyfz4khzl3qfmvcgu.ipfs.localhost/", nil},
		// CIDv1 with long sha512
		{"localhost", "/ipfs/bafkrgqe3ohjcjplc6n4f3fwunlj6upltggn7xqujbsvnvyw764srszz4u4rshq6ztos4chl4plgg4ffyyxnayrtdi5oc4xb2332g645433aeg", "", errors.New("CID incompatible with DNS label length limit of 63: kf1siqrebi3vir8sab33hu5vcy008djegvay6atmz91ojesyjs8lx350b7y7i1nvyw2haytfukfyu2f2x4tocdrfa0zgij6p4zpl4u5oj")},
		// PeerID as CIDv1 needs to have libp2p-key multicodec
		{"localhost", "/ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", "http://k2k4r8n0flx3ra0y5dr8fmyvwbzy3eiztmtq6th694k5a3rznayp3e4o.ipns.localhost/", nil},
		{"localhost", "/ipns/bafybeickencdqw37dpz3ha36ewrh4undfjt2do52chtcky4rxkj447qhdm", "http://k2k4r8l9ja7hkzynavdqup76ou46tnvuaqegbd04a4o1mpbsey0meucb.ipns.localhost/", nil},
		// PeerID: ed25519+identity multihash → CIDv1Base36
		{"localhost", "/ipns/12D3KooWFB51PRY9BxcXSH6khFXw1BZeszeLDy7C8GciskqCTZn5", "http://k51qzi5uqu5di608geewp3nqkg0bpujoasmka7ftkyxgcm3fh1aroup0gsdrna.ipns.localhost/", nil},
		{"sub.localhost", "/ipfs/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", "http://bafybeif7a7gdklt6hodwdrmwmxnhksctcuav6lfxlcyfz4khzl3qfmvcgu.ipfs.sub.localhost/", nil},
	} {
		url, err := toSubdomainURL(test.hostname, test.path, r)
		if url != test.url || !equalError(err, test.err) {
			t.Errorf("(%s, %s) returned (%s, %v), expected (%s, %v)", test.hostname, test.path, url, err, test.url, test.err)
		}
	}
}

func TestHasPrefix(t *testing.T) {
	for _, test := range []struct {
		prefixes []string
		path     string
		out      bool
	}{
		{[]string{"/ipfs"}, "/ipfs/cid", true},
		{[]string{"/ipfs/"}, "/ipfs/cid", true},
		{[]string{"/version/"}, "/version", true},
		{[]string{"/version"}, "/version", true},
	} {
		out := hasPrefix(test.path, test.prefixes...)
		if out != test.out {
			t.Errorf("(%+v, %s) returned '%t', expected '%t'", test.prefixes, test.path, out, test.out)
		}
	}
}

func TestPortStripping(t *testing.T) {
	for _, test := range []struct {
		in  string
		out string
	}{
		{"localhost:8080", "localhost"},
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.localhost:8080", "bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.localhost"},
		{"example.com:443", "example.com"},
		{"example.com", "example.com"},
		{"foo-dweb.ipfs.pvt.k12.ma.us:8080", "foo-dweb.ipfs.pvt.k12.ma.us"},
		{"localhost", "localhost"},
		{"[::1]:8080", "::1"},
	} {
		out := stripPort(test.in)
		if out != test.out {
			t.Errorf("(%s): returned '%s', expected '%s'", test.in, out, test.out)
		}
	}

}

func TestDNSPrefix(t *testing.T) {
	for _, test := range []struct {
		in  string
		out string
		err error
	}{
		// <= 63
		{"QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", "QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", nil},
		{"bafybeickencdqw37dpz3ha36ewrh4undfjt2do52chtcky4rxkj447qhdm", "bafybeickencdqw37dpz3ha36ewrh4undfjt2do52chtcky4rxkj447qhdm", nil},
		// > 63
		// PeerID: ed25519+identity multihash → CIDv1Base36
		{"bafzaajaiaejca4syrpdu6gdx4wsdnokxkprgzxf4wrstuc34gxw5k5jrag2so5gk", "k51qzi5uqu5dj16qyiq0tajolkojyl9qdkr254920wxv7ghtuwcz593tp69z9m", nil},
		// CIDv1 with long sha512 → error
		{"bafkrgqe3ohjcjplc6n4f3fwunlj6upltggn7xqujbsvnvyw764srszz4u4rshq6ztos4chl4plgg4ffyyxnayrtdi5oc4xb2332g645433aeg", "", errors.New("CID incompatible with DNS label length limit of 63: kf1siqrebi3vir8sab33hu5vcy008djegvay6atmz91ojesyjs8lx350b7y7i1nvyw2haytfukfyu2f2x4tocdrfa0zgij6p4zpl4u5oj")},
	} {
		inCID, _ := cid.Decode(test.in)
		out, err := toDNSPrefix(test.in, inCID)
		if out != test.out || !equalError(err, test.err) {
			t.Errorf("(%s): returned (%s, %v) expected (%s, %v)", test.in, out, err, test.out, test.err)
		}
	}

}

func TestKnownSubdomainDetails(t *testing.T) {
	gwLocalhost := &config.GatewaySpec{Paths: []string{"/ipfs", "/ipns", "/api"}, UseSubdomains: true}
	gwDweb := &config.GatewaySpec{Paths: []string{"/ipfs", "/ipns", "/api"}, UseSubdomains: true}
	gwLong := &config.GatewaySpec{Paths: []string{"/ipfs", "/ipns", "/api"}, UseSubdomains: true}
	gwWildcard1 := &config.GatewaySpec{Paths: []string{"/ipfs", "/ipns", "/api"}, UseSubdomains: true}
	gwWildcard2 := &config.GatewaySpec{Paths: []string{"/ipfs", "/ipns", "/api"}, UseSubdomains: true}

	knownGateways := prepareKnownGateways(map[string]*config.GatewaySpec{
		"localhost":               gwLocalhost,
		"dweb.link":               gwDweb,
		"dweb.ipfs.pvt.k12.ma.us": gwLong, // note the sneaky ".ipfs." ;-)
		"*.wildcard1.tld":         gwWildcard1,
		"*.*.wildcard2.tld":       gwWildcard2,
	})

	for _, test := range []struct {
		// in:
		hostHeader string
		// out:
		gw       *config.GatewaySpec
		hostname string
		ns       string
		rootID   string
		ok       bool
	}{
		// no subdomain
		{"127.0.0.1:8080", nil, "", "", "", false},
		{"[::1]:8080", nil, "", "", "", false},
		{"hey.look.example.com", nil, "", "", "", false},
		{"dweb.link", nil, "", "", "", false},
		// malformed Host header
		{".....dweb.link", nil, "", "", "", false},
		{"link", nil, "", "", "", false},
		{"8080:dweb.link", nil, "", "", "", false},
		{" ", nil, "", "", "", false},
		{"", nil, "", "", "", false},
		// unknown gateway host
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.unknown.example.com", nil, "", "", "", false},
		// cid in subdomain, known gateway
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.localhost:8080", gwLocalhost, "localhost:8080", "ipfs", "bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am", true},
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.dweb.link", gwDweb, "dweb.link", "ipfs", "bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am", true},
		// capture everything before .ipfs.
		{"foo.bar.boo-buzz.ipfs.dweb.link", gwDweb, "dweb.link", "ipfs", "foo.bar.boo-buzz", true},
		// ipns
		{"bafzbeihe35nmjqar22thmxsnlsgxppd66pseq6tscs4mo25y55juhh6bju.ipns.localhost:8080", gwLocalhost, "localhost:8080", "ipns", "bafzbeihe35nmjqar22thmxsnlsgxppd66pseq6tscs4mo25y55juhh6bju", true},
		{"bafzbeihe35nmjqar22thmxsnlsgxppd66pseq6tscs4mo25y55juhh6bju.ipns.dweb.link", gwDweb, "dweb.link", "ipns", "bafzbeihe35nmjqar22thmxsnlsgxppd66pseq6tscs4mo25y55juhh6bju", true},
		// edge case check: public gateway under long TLD (see: https://publicsuffix.org)
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.dweb.ipfs.pvt.k12.ma.us", gwLong, "dweb.ipfs.pvt.k12.ma.us", "ipfs", "bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am", true},
		{"bafzbeihe35nmjqar22thmxsnlsgxppd66pseq6tscs4mo25y55juhh6bju.ipns.dweb.ipfs.pvt.k12.ma.us", gwLong, "dweb.ipfs.pvt.k12.ma.us", "ipns", "bafzbeihe35nmjqar22thmxsnlsgxppd66pseq6tscs4mo25y55juhh6bju", true},
		// dnslink in subdomain
		{"en.wikipedia-on-ipfs.org.ipns.localhost:8080", gwLocalhost, "localhost:8080", "ipns", "en.wikipedia-on-ipfs.org", true},
		{"en.wikipedia-on-ipfs.org.ipns.localhost", gwLocalhost, "localhost", "ipns", "en.wikipedia-on-ipfs.org", true},
		{"dist.ipfs.io.ipns.localhost:8080", gwLocalhost, "localhost:8080", "ipns", "dist.ipfs.io", true},
		{"en.wikipedia-on-ipfs.org.ipns.dweb.link", gwDweb, "dweb.link", "ipns", "en.wikipedia-on-ipfs.org", true},
		// edge case check: public gateway under long TLD (see: https://publicsuffix.org)
		{"foo.dweb.ipfs.pvt.k12.ma.us", nil, "", "", "", false},
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.dweb.ipfs.pvt.k12.ma.us", gwLong, "dweb.ipfs.pvt.k12.ma.us", "ipfs", "bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am", true},
		{"bafzbeihe35nmjqar22thmxsnlsgxppd66pseq6tscs4mo25y55juhh6bju.ipns.dweb.ipfs.pvt.k12.ma.us", gwLong, "dweb.ipfs.pvt.k12.ma.us", "ipns", "bafzbeihe35nmjqar22thmxsnlsgxppd66pseq6tscs4mo25y55juhh6bju", true},
		// other namespaces
		{"api.localhost", nil, "", "", "", false},
		{"peerid.p2p.localhost", gwLocalhost, "localhost", "p2p", "peerid", true},
		// wildcards
		{"wildcard1.tld", nil, "", "", "", false},
		{".wildcard1.tld", nil, "", "", "", false},
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.wildcard1.tld", nil, "", "", "", false},
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.sub.wildcard1.tld", gwWildcard1, "sub.wildcard1.tld", "ipfs", "bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am", true},
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.sub1.sub2.wildcard1.tld", nil, "", "", "", false},
		{"bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am.ipfs.sub1.sub2.wildcard2.tld", gwWildcard2, "sub1.sub2.wildcard2.tld", "ipfs", "bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am", true},
	} {
		gw, hostname, ns, rootID, ok := knownSubdomainDetails(test.hostHeader, knownGateways)
		if ok != test.ok {
			t.Errorf("knownSubdomainDetails(%s): ok is %t, expected %t", test.hostHeader, ok, test.ok)
		}
		if rootID != test.rootID {
			t.Errorf("knownSubdomainDetails(%s): rootID is '%s', expected '%s'", test.hostHeader, rootID, test.rootID)
		}
		if ns != test.ns {
			t.Errorf("knownSubdomainDetails(%s): ns is '%s', expected '%s'", test.hostHeader, ns, test.ns)
		}
		if hostname != test.hostname {
			t.Errorf("knownSubdomainDetails(%s): hostname is '%s', expected '%s'", test.hostHeader, hostname, test.hostname)
		}
		if gw != test.gw {
			t.Errorf("knownSubdomainDetails(%s): gw is  %+v, expected %+v", test.hostHeader, gw, test.gw)
		}
	}

}

func equalError(a, b error) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && a.Error() == b.Error())
}
