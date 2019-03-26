// Package dnssec implements a DNSSEC-validating resolver that's capable of
// exporting DNSSEC proofs.
package dnssec

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ipfs/go-ipfs/namesys/dnssec/cache"

	"github.com/miekg/dns"
)

// rootDigests contains identifiers for the current root key-signing keys.
var rootDigests = []*dns.DS{
	&dns.DS{
		Hdr: dns.RR_Header{
			Name:     ".",
			Rrtype:   0x2b,
			Class:    0x01,
			Ttl:      0x1ed8,
			Rdlength: 0x00,
		},
		KeyTag:     0x4a5c,
		Algorithm:  0x08,
		DigestType: 0x02,
		Digest:     "49aac11d7b6f6446702e54a1607371607a1a41855200fd2ce1cdde32f24e8fb5",
	},
	&dns.DS{
		Hdr: dns.RR_Header{
			Name:     ".",
			Rrtype:   0x2b,
			Class:    0x01,
			Ttl:      0x1ed8,
			Rdlength: 0x00,
		},
		KeyTag:     0x4f66,
		Algorithm:  0x08,
		DigestType: 0x02,
		Digest:     "e06d44b80b8f1d39a95c0b0d7c65d08458e880409bbc683457104237c7f8ec8d",
	},
}

type cacheEntry struct {
	msg     *dns.Msg
	signers []string
}

type Resolver struct {
	Cache *cache.Cache
}

func (r *Resolver) LookupA(ctx context.Context, name string) ([]string, *Result, error) {
	res, err := r.lookup(ctx, dns.Fqdn(name), dns.TypeA)
	if err != nil {
		return nil, nil, err
	}
	addrs, err := res.A(name)
	if err != nil {
		return nil, nil, err
	}
	return addrs, res, nil
}

func (r *Resolver) LookupAAAA(ctx context.Context, name string) ([]string, *Result, error) {
	res, err := r.lookup(ctx, dns.Fqdn(name), dns.TypeAAAA)
	if err != nil {
		return nil, nil, err
	}
	addrs, err := res.AAAA(name)
	if err != nil {
		return nil, nil, err
	}
	return addrs, res, nil
}

func (r *Resolver) LookupTXT(ctx context.Context, name string) ([]string, *Result, error) {
	res, err := r.lookup(ctx, dns.Fqdn(name), dns.TypeTXT)
	if err != nil {
		return nil, nil, err
	}
	txts, err := res.TXT(name)
	if err != nil {
		return nil, nil, err
	}
	return txts, res, nil
}

// lookup performs the query and outputs the result along with a DNSSEC proof
// that this result is correct.
func (r *Resolver) lookup(ctx context.Context, name string, qtype uint16) (*Result, error) {
	conn, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	q := &query{
		cache: r.Cache,
		conn:  conn,
	}
	return q.lookup(name, qtype)
}

// connect establishes a reliable connection to a recursive resolver. The
// resolver is expected to do all of the actual heavy DNS lifting.
func (r *Resolver) connect(ctx context.Context) (*dns.Conn, error) {
	client := &dns.Client{
		Net: "tcp",
		Dialer: &net.Dialer{
			Timeout: 2 * time.Second,
			Cancel:  ctx.Done(),
		},
	}
	conn, err := client.Dial("1dot1dot1dot1.cloudflare-dns.com:53")
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(10 * time.Second)
	if sooner, ok := ctx.Deadline(); ok && sooner.Before(deadline) {
		conn.SetDeadline(sooner)
	} else {
		conn.SetDeadline(deadline)
	}

	return conn, nil
}

type query struct {
	cache *cache.Cache
	conn  *dns.Conn

	steps int
	keys  *dns.Msg
	res   *dns.Msg
}

func (q *query) lookup(name string, qtype uint16) (*Result, error) {
	// Get the data the client asked for.
	res, signers, err := q.exchangeOneC(name, qtype)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %v", err)
	}
	q.steps = 0
	q.res = res

	// Foreach candidate signer, fetch their keyset and try to build a
	// chain-of-trust to the root zone that authenticates the response.
	for _, signer := range signers {
		keys, _, err := q.exchangeOneC(signer, dns.TypeDNSKEY)
		if err != nil {
			return nil, fmt.Errorf("failed to get signer's keyset: %v", err)
		}
		q.keys = keys

		var res *Result
		res, err = q.authenticate(signer, nil)
		if err == nil {
			return res, nil
		}
	}

	return nil, err
}

// authenticate is a recursive method that builds a chain-of-trust from signer's
// DS record, up to the root zone.
//
// A DS record may have multiple signers, so we do a depth-first search and
// return the first chain that validates.
func (q *query) authenticate(signer string, delegs []delegMsg) (*Result, error) {
	if signer == "." {
		return newResult(reverseDelegs(delegs), q.keys, q.res)
	}
	const maxSteps = 10
	if q.steps >= maxSteps {
		return nil, fmt.Errorf("tracing the chain of authority took too long")
	}
	q.steps += 1

	deleg, authorities, err := q.exchangeOneC(signer, dns.TypeDS)
	if err != nil {
		return nil, fmt.Errorf("failed to find delegation: %v", err)
	}

	for _, auth := range authorities {
		authKeys, _, err := q.exchangeOneC(auth, dns.TypeDNSKEY)
		if err != nil {
			err = fmt.Errorf("failed to get authority's keyset: %v", err)
			continue
		}
		delegs = append(delegs, delegMsg{authKeys, deleg})

		var res *Result
		res, err = q.authenticate(auth, delegs)
		if err == nil {
			return res, nil
		}

		delegs = delegs[:len(delegs)-1]
	}

	return nil, err
}

// exchangeOneC is a caching wrapper around exchangeOne.
func (q *query) exchangeOneC(name string, qtype uint16) (*dns.Msg, []string, error) {
	if q.cache == nil {
		return q.exchangeOne(name, qtype)
	}
	cacheKey := fmt.Sprintf("%v:%v", name, qtype)

	res, ok := q.cache.Get(cacheKey)
	if ok {
		entry := res.(cacheEntry)
		return entry.msg.Copy(), copySlice(entry.signers), nil
	}

	msg, signers, err := q.exchangeOne(name, qtype)
	if err != nil {
		return nil, nil, err
	}
	q.cache.Set(cacheKey, cacheEntry{msg, signers}, cache.DefaultExpiration)

	return msg.Copy(), copySlice(signers), nil
}

// exchangeOne sends a question to the resolver at `conn` and reads the
// response. It checks that the response is well-formed and signed (the
// signature is not verified). It returns the resolver's response and the
// de-duplicated names of the signers.
func (q *query) exchangeOne(name string, qtype uint16) (*dns.Msg, []string, error) {
	req := new(dns.Msg)
	req.SetQuestion(name, qtype)
	req.SetEdns0(4096, true) // Tell the nameserver we support DNSSEC.

	err := q.conn.WriteMsg(req)
	if err != nil {
		return nil, nil, err
	}
	res, err := q.conn.ReadMsg()
	if err != nil {
		return nil, nil, err
	} else if res.Id != req.Id {
		return nil, nil, dns.ErrId
	} else if res.Rcode != dns.RcodeSuccess {
		return nil, nil, fmt.Errorf("unexpected response code (%v)", res.Rcode)
	} else if len(res.Ns) > 0 {
		return nil, nil, fmt.Errorf("response has unexpected records in authority section (Is TXT record with _dnslink. prefix set?)")
	}

	// Verify that the response we got back has: some of the records of the type
	// we asked for, a signature over those records, and nothing else.
	var signers []string
	hasResp, hasSig := false, false

	for _, rr := range res.Answer {
		if sig, ok := rr.(*dns.RRSIG); ok {
			// This is a signature record; store the signer name, if it's not a
			// duplicate.
			found := false
			for _, cand := range signers {
				if sig.SignerName == cand {
					found = true
				}
			}
			if !found {
				signers = append(signers, sig.SignerName)
			}
			hasSig = true
			continue
		}

		hdr := rr.Header()
		if hdr.Rrtype != qtype {
			return nil, nil, fmt.Errorf("response has unexpected record type: %T (%v)", rr, hdr.Rrtype)
		}
		hasResp = true
	}

	if !hasResp {
		return nil, nil, fmt.Errorf("response has no records of the requested type (%v)", qtype)
	} else if !hasSig {
		return nil, nil, fmt.Errorf("response is not signed (Is DNSSEC configured?)")
	}
	return res, signers, nil
}

func reverseDelegs(in []delegMsg) []delegMsg {
	if in == nil {
		return nil
	}
	out := make([]delegMsg, len(in))
	for i := 0; i < len(in); i++ {
		out[i] = in[len(in)-i-1]
	}
	return out
}

func copySlice(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
