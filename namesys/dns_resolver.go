package namesys

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/miekg/dns"
	cacheLib "github.com/patrickmn/go-cache"
	"golang.org/x/net/http2"
)

// Class managing custom DNS resolvers
type customDNS struct {
	// Address is the address of the DNS server (e.g. 1.1.1.1)
	Address string

	// Protocol to use: "udp" (or "" - default), "dns-over-https", "dns-over-tls"
	// Optional: default is "" (= "udp")
	Protocol string

	// DNSoverHTTPSHost is the value for the "Host" header used when making requests via DNS-over-HTTPS
	// Optional: use if necessary
	DNSoverHTTPSHost string

	// Port is the port the DNS server listens to
	// Optional: default value is 53 (udp), 443 (dns-over-https) or 853 (dns-over-tls) depending on the protocol used
	Port uint
}

// Timeout for DNS requests
const reqTimeout = 5 * time.Second

// Interval between DNS cache purges
const dnsCachePurgeInterval = 5 * time.Minute

// Initialize the cache
var cache = cacheLib.New(dnsCachePurgeInterval, dnsCachePurgeInterval)

// LookupTXT looks up a TXT record, using the cache when available
func (d *customDNS) LookupTXT(name string) (txt []string, err error) {
	// Check if the record is available in cache
	if msgI, found := cache.Get(name); found {
		fmt.Println("Responding from cache")
		msg := msgI.(*dns.Msg)
		// We already checked and are sure that this response exists
		t := msg.Answer[0].(*dns.TXT)
		txt = t.Txt
		return
	}

	txt, err = d.RequestTXT(name)
	return
}

// RequestTXT performs the request for the TXT record to the server
func (d *customDNS) RequestTXT(name string) (txt []string, err error) {
	// Request
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeTXT)

	// Response
	var in *dns.Msg

	// dns-over-https has a different path
	if d.Protocol == "dns-over-https" {
		// Request
		in, err = d.ExchangeDoHRequest(m, name)
		if err != nil {
			return
		}
	} else { // dns-over-tls and standard
		// Request
		in, err = d.ExchangeDNSRequest(m, name)
		if err != nil {
			return
		}
	}

	// Get the TXT record
	if t, ok := in.Answer[0].(*dns.TXT); ok {
		txt = t.Txt

		// Check if we have a TTL
		if t.Hdr.Ttl > 0 {
			// Cache the result
			d.cacheResult(name, in)
		}
	}

	return
}

// ExchangeDNSRequest sends a request using the DNS UDP protocol or DNS-over-TLS
func (d *customDNS) ExchangeDNSRequest(msg *dns.Msg, name string) (in *dns.Msg, err error) {
	// Create the DNS client with the correct transport, then set the port
	var c dns.Client
	port := d.Port
	if d.Protocol == "dns-over-tls" {
		c = dns.Client{
			Net:         "tcp-tls",
			DialTimeout: reqTimeout,
		}

		if port == 0 {
			port = 853
		}
	} else {
		c = dns.Client{}

		if port == 0 {
			port = 53
		}
	}

	// Address
	addr := fmt.Sprintf("%s:%d", d.Address, port)
	fmt.Println("Resolving TXT for", name, "using server", addr)

	// Send the request
	var rtt time.Duration
	in, rtt, err = c.Exchange(msg, addr)
	if err != nil {
		return
	}
	fmt.Println("Request took", rtt)
	return
}

// ExchangeDoHRequest sends a request using the DNS-over-HTTPS protocol
func (d *customDNS) ExchangeDoHRequest(msg *dns.Msg, name string) (in *dns.Msg, err error) {
	// Get the port
	port := d.Port
	if port == 0 {
		port = 443
	}

	// Initialize the HTTP client
	tlsConf := &tls.Config{}
	if len(d.DNSoverHTTPSHost) > 0 {
		tlsConf.ServerName = d.DNSoverHTTPSHost
	}
	client := &http.Client{
		Timeout: reqTimeout,
		Transport: &http2.Transport{
			DisableCompression: true,
			TLSClientConfig:    tlsConf,
		},
	}

	// Serialize the message
	var body []byte
	body, err = msg.Pack()
	if err != nil {
		return
	}

	// Create a POST request
	var req *http.Request
	req, err = http.NewRequest("POST", fmt.Sprintf("https://%s:%d/dns-query", d.Address, port), bytes.NewBuffer(body))
	if err != nil {
		return
	}

	// Set headers
	if len(d.DNSoverHTTPSHost) > 0 {
		req.Host = d.DNSoverHTTPSHost
	}
	req.Header.Set("Content-Type", "application/dns-message")

	var res *http.Response
	res, err = client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Invalid response status code: %d", res.StatusCode)
	}

	var raw []byte
	raw, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	if len(raw) < 1 {
		err = fmt.Errorf("Response is empty")
		return
	}

	// Parse the response
	in = &dns.Msg{}
	err = in.Unpack(raw)
	if err != nil {
		in = nil
		return
	}

	return
}

func (d *customDNS) cacheResult(name string, msg *dns.Msg) {
	ttl := time.Duration(msg.Answer[0].Header().Ttl)
	cache.Set(name, msg, ttl*time.Second)
}
