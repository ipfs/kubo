package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
)

var (
	ctx         = context.Background()
	testAddr, _ = ma.NewMultiaddr("/dns4/example.com/tcp/5001")
)

func makeResolver(n uint8) *madns.Resolver {
	results := make([]net.IPAddr, n)
	for i := uint8(0); i < n; i++ {
		results[i] = net.IPAddr{IP: net.ParseIP(fmt.Sprintf("192.0.2.%d", i))}
	}

	backend := &madns.MockBackend{
		IP: map[string][]net.IPAddr{
			"example.com": results,
		}}

	return &madns.Resolver{
		Backend: backend,
	}
}

func TestApiEndpointResolveDNSOneResult(t *testing.T) {
	dnsResolver = makeResolver(1)

	addr, err := resolveAddr(ctx, testAddr)
	if err != nil {
		t.Error(err)
	}

	if ref, _ := ma.NewMultiaddr("/ip4/192.0.2.0/tcp/5001"); !addr.Equal(ref) {
		t.Errorf("resolved address was different than expected")
	}
}

func TestApiEndpointResolveDNSMultipleResults(t *testing.T) {
	dnsResolver = makeResolver(4)

	addr, err := resolveAddr(ctx, testAddr)
	if err != nil {
		t.Error(err)
	}

	if ref, _ := ma.NewMultiaddr("/ip4/192.0.2.0/tcp/5001"); !addr.Equal(ref) {
		t.Errorf("resolved address was different than expected")
	}
}

func TestApiEndpointResolveDNSNoResults(t *testing.T) {
	dnsResolver = makeResolver(0)

	addr, err := resolveAddr(ctx, testAddr)
	if addr != nil || err == nil {
		t.Error("expected test address not to resolve, and to throw an error")
	}

	if !strings.HasPrefix(err.Error(), "non-resolvable API endpoint") {
		t.Errorf("expected error not thrown; actual: %v", err)
	}
}
