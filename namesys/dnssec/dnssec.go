package dnssec

import (
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func supportedAlg(alg uint8) bool {
	return alg == 8 || alg == 13
}

// chooseKeyset takes the response from a DNSKEY query `msg` and returns the
// DNSKEY RRs from inside, along with the preferred signature over the RRs.
//
// The preferred signature is guaranteed to verify against a public key
// referenced by one of the DS RRs in `digests`.
func chooseKeyset(digests []*dns.DS, msg *dns.Msg) ([]*dns.DNSKEY, *dns.RRSIG, error) {
	keys := make([]*dns.DNSKEY, 0)
	for _, rr := range msg.Answer {
		key, ok := rr.(*dns.DNSKEY)
		if !ok {
			continue
		}
		keys = append(keys, key)
	}

	for _, rr := range msg.Answer {
		cand, ok := rr.(*dns.RRSIG)
		if !ok {
			continue
		} else if !supportedAlg(cand.Algorithm) {
			continue
		} else if err := verifyKeyset(digests, keys, cand); err != nil {
			continue
		}

		return keys, cand, nil
	}

	return nil, nil, fmt.Errorf("no suitable signatures over the authority's keyset were found")
}

// verifyKeyset returns an error unless there is a public key in `keys` that
// `sig` will verify against, and that this public key is referenced by a DS RR
// in `digests`.
func verifyKeyset(digests []*dns.DS, keys []*dns.DNSKEY, sig *dns.RRSIG) (err error) {
	if !sig.ValidityPeriod(time.Time{}) {
		return fmt.Errorf("signature is not currently valid")
	}

	recs := make([]dns.RR, 0, len(keys))
	for _, rr := range keys {
		recs = append(recs, rr)
	}

	for _, key := range keys {
		if key.Flags&dns.ZONE == 0 {
			continue
		}
		cand := key.ToDS(dns.SHA256)

		for _, ds := range digests {
			matches := ds.KeyTag == cand.KeyTag &&
				ds.Algorithm == cand.Algorithm &&
				ds.DigestType == cand.DigestType &&
				ds.Digest == cand.Digest &&
				strings.ToLower(ds.Hdr.Name) == strings.ToLower(key.Hdr.Name)
			if !matches {
				continue
			}
			err = sig.Verify(key, recs)
			if err != nil {
				continue
			}
			return nil
		}
	}

	if err == nil {
		err = fmt.Errorf("no suitable signatures over the authority's keyset were found")
	}
	return err
}

// chooseRecs takes a query response `msg` and returns the non-signature RRs
// from inside, along with the preferred signature over the RRs. The preferred
// signature verifies against one of the public keys in `keys`.
func chooseRecs(keys []*dns.DNSKEY, msg *dns.Msg) ([]dns.RR, *dns.RRSIG, error) {
	recs := make([]dns.RR, 0)
	for _, rr := range msg.Answer {
		if _, ok := rr.(*dns.RRSIG); ok {
			continue
		}
		recs = append(recs, rr)
	}

	for _, rr := range msg.Answer {
		sig, ok := rr.(*dns.RRSIG)
		if !ok {
			continue
		} else if !supportedAlg(sig.Algorithm) {
			continue
		} else if err := verifyRecs(keys, recs, sig); err != nil {
			continue
		}

		return recs, sig, nil
	}

	return nil, nil, fmt.Errorf("no suitable signatures over the resource record set were found")
}

// verifyRecs returns an error unless there is a public key in `keys` that `sig`
// will verify against.
func verifyRecs(keys []*dns.DNSKEY, recs []dns.RR, sig *dns.RRSIG) (err error) {
	if !sig.ValidityPeriod(time.Time{}) {
		return fmt.Errorf("signature is not currently valid")
	}

	for _, key := range keys {
		if key.Flags&dns.ZONE == 0 {
			continue
		} else if !suffixed(recs, key.Hdr.Name) {
			continue
		}
		err = sig.Verify(key, recs)
		if err != nil {
			continue
		}
		return nil
	}

	if err == nil {
		err = fmt.Errorf("no suitable signatures over the resource record set were found")
	}
	return err
}

// suffixed returns true if every resource record in `recs` is a child of the
// given parent zone.
func suffixed(recs []dns.RR, parent string) bool {
	parent = strings.ToLower(parent)

	suffix := parent
	if suffix != "." {
		suffix = "." + suffix
	}

	for _, rr := range recs {
		name := strings.ToLower(rr.Header().Name)
		if name != parent && !strings.HasSuffix(name, suffix) {
			return false
		}
	}

	return true
}
