package main

import (
	"testing"

	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations/ipfsfetcher"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

var testPeerStrs = []string{
	"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5",
	"/ip4/127.0.0.1/udp/4001/quic/p2p/12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ7",
}

var testPeers []peer.AddrInfo

func init() {
	var err error
	testPeers, err = parsePeers(testPeerStrs)
	if err != nil {
		panic(err)
	}
}

func TestGetMigrationFetcher(t *testing.T) {
	var f migrations.Fetcher
	var err error
	_, err = getMigrationFetcher([]string{"ftp://bad.gateway.io"}, nil)
	if err == nil {
		t.Fatal("Expected bad URL scheme error")
	}

	f, err = getMigrationFetcher([]string{"ipfs"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*ipfsfetcher.IpfsFetcher); !ok {
		t.Fatal("expected IpfsFetcher")
	}

	f, err = getMigrationFetcher([]string{"http"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*migrations.HttpFetcher); !ok {
		t.Fatal("expected HttpFetcher")
	}

	f, err = getMigrationFetcher([]string{"IPFS", "HTTPS"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mf, ok := f.(*migrations.MultiFetcher)
	if !ok {
		t.Fatal("expected MultiFetcher")
	}
	if mf.Len() != 2 {
		t.Fatal("expected 2 fetchers in MultiFetcher")
	}

	f, err = getMigrationFetcher([]string{"ipfs", "https", "some.domain.io"}, testPeers)
	if err != nil {
		t.Fatal(err)
	}
	mf, ok = f.(*migrations.MultiFetcher)
	if !ok {
		t.Fatal("expected MultiFetcher")
	}
	if mf.Len() != 3 {
		t.Fatal("expected 3 fetchers in MultiFetcher")
	}

	_, err = getMigrationFetcher(nil, nil)
	if err == nil {
		t.Fatal("expected error when no fetchers specified")
	}

	_, err = getMigrationFetcher([]string{"", ""}, nil)
	if err == nil {
		t.Fatal("expected error when empty string fetchers specified")
	}
}

// parsePeers parses multiaddr strings in the form:
// /<ip-proto>/<ip-addr>/<transport>/<port>/p2p/<node-id>
func parsePeers(peers []string) ([]peer.AddrInfo, error) {
	if len(peers) == 0 {
		return nil, nil
	}

	// Parse the peer addresses
	pinfos := make(map[peer.ID]*peer.AddrInfo, len(peers))
	for _, addrStr := range peers {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			return nil, err
		}
		pii, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return nil, err
		}
		pi, ok := pinfos[pii.ID]
		if !ok {
			pi = &peer.AddrInfo{ID: pii.ID}
			pinfos[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}
	peerAddrs := make([]peer.AddrInfo, len(pinfos))
	var i int
	for _, pi := range pinfos {
		peerAddrs[i] = peer.AddrInfo{
			ID:    pi.ID,
			Addrs: pi.Addrs,
		}
		i++
	}

	return peerAddrs, nil
}
