package autoconf

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoConfDNS(t *testing.T) {
	t.Parallel()

	t.Run("DNS resolution with auto DoH resolver", func(t *testing.T) {
		t.Parallel()
		testDNSResolutionWithAutoDoH(t)
	})

	t.Run("DNS errors are handled properly", func(t *testing.T) {
		t.Parallel()
		testDNSErrorHandling(t)
	})
}

// mockDoHServer implements a simple DNS-over-HTTPS server for testing
type mockDoHServer struct {
	t            *testing.T
	server       *httptest.Server
	mu           sync.Mutex
	requests     []string
	responseFunc func(name string) *dns.Msg
}

func newMockDoHServer(t *testing.T) *mockDoHServer {
	m := &mockDoHServer{
		t:        t,
		requests: []string{},
	}

	// Default response function returns a dnslink TXT record
	m.responseFunc = func(name string) *dns.Msg {
		msg := &dns.Msg{}
		msg.SetReply(&dns.Msg{Question: []dns.Question{{Name: name, Qtype: dns.TypeTXT}}})

		if strings.HasPrefix(name, "_dnslink.") {
			// Return a valid dnslink record
			rr := &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Txt: []string{"dnslink=/ipfs/QmYNQJoKGNHTpPxCBPh9KkDpaExgd2duMa3aF6ytMpHdao"},
			}
			msg.Answer = append(msg.Answer, rr)
		}

		return msg
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/dns-query", m.handleDNSQuery)

	m.server = httptest.NewServer(mux)
	return m
}

func (m *mockDoHServer) handleDNSQuery(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var dnsMsg *dns.Msg

	if r.Method == "GET" {
		// Handle GET with ?dns= parameter
		dnsParam := r.URL.Query().Get("dns")
		if dnsParam == "" {
			http.Error(w, "missing dns parameter", http.StatusBadRequest)
			return
		}

		data, err := base64.RawURLEncoding.DecodeString(dnsParam)
		if err != nil {
			http.Error(w, "invalid base64", http.StatusBadRequest)
			return
		}

		dnsMsg = &dns.Msg{}
		if err := dnsMsg.Unpack(data); err != nil {
			http.Error(w, "invalid DNS message", http.StatusBadRequest)
			return
		}
	} else if r.Method == "POST" {
		// Handle POST with DNS wire format
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		dnsMsg = &dns.Msg{}
		if err := dnsMsg.Unpack(data); err != nil {
			http.Error(w, "invalid DNS message", http.StatusBadRequest)
			return
		}
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Log the DNS query
	if len(dnsMsg.Question) > 0 {
		qname := dnsMsg.Question[0].Name
		m.requests = append(m.requests, qname)
		m.t.Logf("DoH server received query for: %s", qname)
	}

	// Generate response
	response := m.responseFunc(dnsMsg.Question[0].Name)
	responseData, err := response.Pack()
	if err != nil {
		http.Error(w, "failed to pack response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/dns-message")
	_, _ = w.Write(responseData)
}

func (m *mockDoHServer) getRequests() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.requests...)
}

func (m *mockDoHServer) close() {
	m.server.Close()
}

func testDNSResolutionWithAutoDoH(t *testing.T) {
	// Create mock DoH server
	dohServer := newMockDoHServer(t)
	defer dohServer.close()

	// Create autoconf data with DoH resolver for "foo." domain
	autoConfData := fmt.Sprintf(`{
		"AutoConfVersion": 2025072302,
		"AutoConfSchema": 1,
		"AutoConfTTL": 86400,
		"SystemRegistry": {
			"AminoDHT": {
				"Description": "Test AminoDHT system",
				"NativeConfig": {
					"Bootstrap": []
				}
			}
		},
		"DNSResolvers": {
			"foo.": ["%s/dns-query"]
		},
		"DelegatedEndpoints": {}
	}`, dohServer.server.URL)

	// Create autoconf server
	autoConfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(autoConfData))
	}))
	defer autoConfServer.Close()

	// Create IPFS node with auto DNS resolver
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", autoConfServer.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})

	// Start daemon
	node.StartDaemon()
	defer node.StopDaemon()

	// Verify config still shows "auto" for DNS resolvers
	result := node.RunIPFS("config", "DNS.Resolvers")
	require.Equal(t, 0, result.ExitCode())
	dnsResolversOutput := result.Stdout.String()
	assert.Contains(t, dnsResolversOutput, "foo.", "DNS resolvers should contain foo. domain")
	assert.Contains(t, dnsResolversOutput, "auto", "DNS resolver config should show 'auto'")

	// Try to resolve a .foo domain
	result = node.RunIPFS("resolve", "/ipns/example.foo")
	require.Equal(t, 0, result.ExitCode())

	// Should resolve to the IPFS path from our mock DoH server
	output := strings.TrimSpace(result.Stdout.String())
	assert.Equal(t, "/ipfs/QmYNQJoKGNHTpPxCBPh9KkDpaExgd2duMa3aF6ytMpHdao", output,
		"Should resolve to the path returned by DoH server")

	// Verify DoH server received the DNS query
	requests := dohServer.getRequests()
	require.Greater(t, len(requests), 0, "DoH server should have received at least one request")

	foundDNSLink := false
	for _, req := range requests {
		if strings.Contains(req, "_dnslink.example.foo") {
			foundDNSLink = true
			break
		}
	}
	assert.True(t, foundDNSLink, "DoH server should have received query for _dnslink.example.foo")
}

func testDNSErrorHandling(t *testing.T) {
	// Create DoH server that returns NXDOMAIN
	dohServer := newMockDoHServer(t)
	defer dohServer.close()

	// Configure to return NXDOMAIN
	dohServer.responseFunc = func(name string) *dns.Msg {
		msg := &dns.Msg{}
		msg.SetReply(&dns.Msg{Question: []dns.Question{{Name: name, Qtype: dns.TypeTXT}}})
		msg.Rcode = dns.RcodeNameError // NXDOMAIN
		return msg
	}

	// Create autoconf data with DoH resolver
	autoConfData := fmt.Sprintf(`{
		"AutoConfVersion": 2025072302,
		"AutoConfSchema": 1,
		"AutoConfTTL": 86400,
		"SystemRegistry": {
			"AminoDHT": {
				"Description": "Test AminoDHT system",
				"NativeConfig": {
					"Bootstrap": []
				}
			}
		},
		"DNSResolvers": {
			"bar.": ["%s/dns-query"]
		},
		"DelegatedEndpoints": {}
	}`, dohServer.server.URL)

	// Create autoconf server
	autoConfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(autoConfData))
	}))
	defer autoConfServer.Close()

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", autoConfServer.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"bar.": "auto"})

	// Start daemon
	node.StartDaemon()
	defer node.StopDaemon()

	// Try to resolve a non-existent domain
	result := node.RunIPFS("resolve", "/ipns/nonexistent.bar")
	require.NotEqual(t, 0, result.ExitCode(), "Resolution should fail for non-existent domain")

	// Should contain appropriate error message
	stderr := result.Stderr.String()
	assert.Contains(t, stderr, "could not resolve name",
		"Error should indicate DNS resolution failure")

	// Verify DoH server received the query
	requests := dohServer.getRequests()
	foundQuery := false
	for _, req := range requests {
		if strings.Contains(req, "_dnslink.nonexistent.bar") {
			foundQuery = true
			break
		}
	}
	assert.True(t, foundQuery, "DoH server should have received query even for failed resolution")
}
