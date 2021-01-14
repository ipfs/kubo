package corehttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	cid "github.com/ipfs/go-cid"
	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	path "github.com/ipfs/go-path"
)

func TestToSubdomainURL(t *testing.T) {
	ns := mockNamesys{}
	n, err := newNodeWithMockNamesys(ns)
	if err != nil {
		t.Fatal(err)
	}
	coreAPI, err := coreapi.NewCoreAPI(n)
	if err != nil {
		t.Fatal(err)
	}
	testCID, err := coreAPI.Unixfs().Add(n.Context(), files.NewBytesFile([]byte("fnord")))
	if err != nil {
		t.Fatal(err)
	}
	ns["/ipns/dnslink.long-name.example.com"] = path.FromString(testCID.String())
	ns["/ipns/dnslink.too-long.f1siqrebi3vir8sab33hu5vcy008djegvay6atmz91ojesyjs8lx350b7y7i1nvyw2haytfukfyu2f2x4tocdrfa0zgij6p4zpl4u5o.example.com"] = path.FromString(testCID.String())
	httpRequest := httptest.NewRequest("GET", "http://127.0.0.1:8080", nil)
	httpsRequest := httptest.NewRequest("GET", "https://https-request-stub.example.com", nil)
	httpsProxiedRequest := httptest.NewRequest("GET", "http://proxied-https-request-stub.example.com", nil)
	httpsProxiedRequest.Header.Set("X-Forwarded-Proto", "https")

	for _, test := range []struct {
		// in:
		request    *http.Request
		gwHostname string
		path       string
		// out:
		url string
		err error
	}{
		// DNSLink
		{httpRequest, "localhost", "/ipns/dnslink.io", "http://dnslink.io.ipns.localhost/", nil},
		// Hostname with port
		{httpRequest, "localhost:8080", "/ipns/dnslink.io", "http://dnslink.io.ipns.localhost:8080/", nil},
		// CIDv0 → CIDv1base32
		{httpRequest, "localhost", "/ipfs/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", "http://bafybeif7a7gdklt6hodwdrmwmxnhksctcuav6lfxlcyfz4khzl3qfmvcgu.ipfs.localhost/", nil},
		// CIDv1 with long sha512
		{httpRequest, "localhost", "/ipfs/bafkrgqe3ohjcjplc6n4f3fwunlj6upltggn7xqujbsvnvyw764srszz4u4rshq6ztos4chl4plgg4ffyyxnayrtdi5oc4xb2332g645433aeg", "", errors.New("CID incompatible with DNS label length limit of 63: kf1siqrebi3vir8sab33hu5vcy008djegvay6atmz91ojesyjs8lx350b7y7i1nvyw2haytfukfyu2f2x4tocdrfa0zgij6p4zpl4u5oj")},
		// PeerID as CIDv1 needs to have libp2p-key multicodec
		{httpRequest, "localhost", "/ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", "http://k2k4r8n0flx3ra0y5dr8fmyvwbzy3eiztmtq6th694k5a3rznayp3e4o.ipns.localhost/", nil},
		{httpRequest, "localhost", "/ipns/bafybeickencdqw37dpz3ha36ewrh4undfjt2do52chtcky4rxkj447qhdm", "http://k2k4r8l9ja7hkzynavdqup76ou46tnvuaqegbd04a4o1mpbsey0meucb.ipns.localhost/", nil},
		// PeerID: ed25519+identity multihash → CIDv1Base36
		{httpRequest, "localhost", "/ipns/12D3KooWFB51PRY9BxcXSH6khFXw1BZeszeLDy7C8GciskqCTZn5", "http://k51qzi5uqu5di608geewp3nqkg0bpujoasmka7ftkyxgcm3fh1aroup0gsdrna.ipns.localhost/", nil},
		{httpRequest, "sub.localhost", "/ipfs/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", "http://bafybeif7a7gdklt6hodwdrmwmxnhksctcuav6lfxlcyfz4khzl3qfmvcgu.ipfs.sub.localhost/", nil},
		// HTTPS requires DNSLink name to fit in a single DNS label – see "Option C" from https://github.com/ipfs/in-web-browsers/issues/169
		{httpRequest, "dweb.link", "/ipns/dnslink.long-name.example.com", "http://dnslink.long-name.example.com.ipns.dweb.link/", nil},
		{httpsRequest, "dweb.link", "/ipns/dnslink.long-name.example.com", "https://dnslink-long--name-example-com.ipns.dweb.link/", nil},
		{httpsProxiedRequest, "dweb.link", "/ipns/dnslink.long-name.example.com", "https://dnslink-long--name-example-com.ipns.dweb.link/", nil},
	} {
		url, err := toSubdomainURL(test.gwHostname, test.path, test.request, coreAPI)
		if url != test.url || !equalError(err, test.err) {
			t.Errorf("(%s, %s) returned (%s, %v), expected (%s, %v)", test.gwHostname, test.path, url, err, test.url, test.err)
		}
	}
}

func TestToDNSLinkDNSLabel(t *testing.T) {
	for _, test := range []struct {
		in  string
		out string
		err error
	}{
		{"dnslink.long-name.example.com", "dnslink-long--name-example-com", nil},
		{"dnslink.too-long.f1siqrebi3vir8sab33hu5vcy008djegvay6atmz91ojesyjs8lx350b7y7i1nvyw2haytfukfyu2f2x4tocdrfa0zgij6p4zpl4u5o.example.com", "", errors.New("DNSLink representation incompatible with DNS label length limit of 63: dnslink-too--long-f1siqrebi3vir8sab33hu5vcy008djegvay6atmz91ojesyjs8lx350b7y7i1nvyw2haytfukfyu2f2x4tocdrfa0zgij6p4zpl4u5o-example-com")},
	} {
		out, err := toDNSLinkDNSLabel(test.in)
		if out != test.out || !equalError(err, test.err) {
			t.Errorf("(%s) returned (%s, %v), expected (%s, %v)", test.in, out, err, test.out, test.err)
		}
	}
}

func TestToDNSLinkFQDN(t *testing.T) {
	for _, test := range []struct {
		in  string
		out string
	}{
		{"singlelabel", "singlelabel"},
		{"docs-ipfs-io", "docs.ipfs.io"},
		{"dnslink-long--name-example-com", "dnslink.long-name.example.com"},
	} {
		out := toDNSLinkFQDN(test.in)
		if out != test.out {
			t.Errorf("(%s) returned (%s), expected (%s)", test.in, out, test.out)
		}
	}
}

func TestIsHTTPSRequest(t *testing.T) {
	httpRequest := httptest.NewRequest("GET", "http://127.0.0.1:8080", nil)
	httpsRequest := httptest.NewRequest("GET", "https://https-request-stub.example.com", nil)
	httpsProxiedRequest := httptest.NewRequest("GET", "http://proxied-https-request-stub.example.com", nil)
	httpsProxiedRequest.Header.Set("X-Forwarded-Proto", "https")
	httpProxiedRequest := httptest.NewRequest("GET", "http://proxied-http-request-stub.example.com", nil)
	httpProxiedRequest.Header.Set("X-Forwarded-Proto", "http")
	oddballRequest := httptest.NewRequest("GET", "foo://127.0.0.1:8080", nil)
	for _, test := range []struct {
		in  *http.Request
		out bool
	}{
		{httpRequest, false},
		{httpsRequest, true},
		{httpsProxiedRequest, true},
		{httpProxiedRequest, false},
		{oddballRequest, false},
	} {
		out := isHTTPSRequest(test.in)
		if out != test.out {
			t.Errorf("(%+v): returned %t, expected %t", test.in, out, test.out)
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

func TestToDNSLabel(t *testing.T) {
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
		out, err := toDNSLabel(test.in, inCID)
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
