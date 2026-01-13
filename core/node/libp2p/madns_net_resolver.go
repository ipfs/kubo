package libp2p

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	madns "github.com/multiformats/go-multiaddr-dns"
)

// NewNetResolverFromMadns creates a *net.Resolver that uses madns.Resolver internally.
// This allows p2p-forge to use DNS.Resolvers config for ACME DNS-01 self-checks.
func NewNetResolverFromMadns(resolver *madns.Resolver) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return &madnsProxyConn{
				resolver: resolver,
				ctx:      ctx,
			}, nil
		},
	}
}

// madnsProxyConn implements net.Conn by proxying DNS queries to madns.Resolver.
// It intercepts DNS wire protocol, parses queries, calls the madns resolver,
// and returns properly formatted DNS responses.
type madnsProxyConn struct {
	resolver *madns.Resolver
	ctx      context.Context
	resp     bytes.Buffer
}

func (c *madnsProxyConn) Write(p []byte) (int, error) {
	c.resp.Reset()

	// Go's net.Resolver with PreferGo=true uses TCP-style messages
	// with 2-byte length prefix even for "udp" network
	var queryData []byte
	if len(p) >= 2 {
		length := int(binary.BigEndian.Uint16(p[:2]))
		if len(p) >= 2+length {
			queryData = p[2 : 2+length]
		} else {
			queryData = p[2:] // partial data
		}
	} else {
		queryData = p
	}

	if len(queryData) == 0 {
		return len(p), nil
	}

	// Parse DNS message
	var msg dns.Msg
	if err := msg.Unpack(queryData); err != nil {
		// Return len(p) to indicate we consumed the data, but don't fail
		// The response buffer will be empty, causing Read to return EOF
		return len(p), nil
	}

	// Build response
	resp := &dns.Msg{}
	resp.SetReply(&msg)
	resp.Authoritative = true // Prevents "lame referral" errors

	for _, q := range msg.Question {
		name := strings.TrimSuffix(q.Name, ".")
		switch q.Qtype {
		case dns.TypeTXT:
			records, err := c.resolver.LookupTXT(c.ctx, name)
			if err == nil {
				for _, txt := range records {
					resp.Answer = append(resp.Answer, &dns.TXT{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 300},
						Txt: []string{txt},
					})
				}
			}
		case dns.TypeA:
			addrs, err := c.resolver.LookupIPAddr(c.ctx, name)
			if err == nil {
				for _, addr := range addrs {
					if ipv4 := addr.IP.To4(); ipv4 != nil {
						resp.Answer = append(resp.Answer, &dns.A{
							Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
							A:   ipv4,
						})
					}
				}
			}
		case dns.TypeAAAA:
			addrs, err := c.resolver.LookupIPAddr(c.ctx, name)
			if err == nil {
				for _, addr := range addrs {
					if addr.IP.To4() == nil && addr.IP.To16() != nil {
						resp.Answer = append(resp.Answer, &dns.AAAA{
							Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
							AAAA: addr.IP,
						})
					}
				}
			}
		default:
			// Unsupported query type - return empty response (NODATA)
		}
	}

	// Pack response
	respData, err := resp.Pack()
	if err != nil {
		return len(p), err
	}

	// Go's pure-Go resolver (PreferGo=true) always uses TCP-style length prefix
	// Write 2-byte big-endian length, then the response data
	lengthBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthBuf, uint16(len(respData)))
	c.resp.Write(lengthBuf)
	c.resp.Write(respData)

	return len(p), nil
}

func (c *madnsProxyConn) Read(p []byte) (int, error) {
	return c.resp.Read(p)
}

func (c *madnsProxyConn) Close() error                       { return nil }
func (c *madnsProxyConn) LocalAddr() net.Addr                { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
func (c *madnsProxyConn) RemoteAddr() net.Addr               { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
func (c *madnsProxyConn) SetDeadline(t time.Time) error      { return nil }
func (c *madnsProxyConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *madnsProxyConn) SetWriteDeadline(t time.Time) error { return nil }
