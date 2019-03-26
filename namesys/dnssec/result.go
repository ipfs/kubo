package dnssec

import (
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/miekg/dns"

	"github.com/ipfs/go-ipfs/namesys/dnssec/pb"
)

//go:generate protoc --go_out=. pb/result.proto

type delegMsg struct {
	keys    *dns.Msg // keys contains the signed DNSKEY records for the current authority.
	digests *dns.Msg // digests contains the signed DS records for the next authority.
}

// Result wraps the output of a DNS query with the cryptographic material
// necessary to verify the output's validity.
//
// This data is meant to be serialized and sent over the wire to a client that
// can't do secure DNS resolution. They can then parse it and call
// Result.Verify() to see that this is correct. To extract the response data,
// they'd call the relevant method with the zone name they expect records for.
type Result struct {
	Delegations []Delegation

	Keys []*dns.DNSKEY
	Data []dns.RR

	KeySig, DataSig *dns.RRSIG
}

func newResult(delegMsgs []delegMsg, keyMsg, resMsg *dns.Msg) (*Result, error) {
	delegs := make([]Delegation, 0, len(delegMsgs))
	digests := rootDigests

	for _, deleg := range delegMsgs {
		d, err := newDelegation(digests, deleg)
		if err != nil {
			return nil, err
		}
		delegs = append(delegs, *d)
		digests = d.Digests
	}

	keys, keySig, err := chooseKeyset(digests, keyMsg)
	if err != nil {
		return nil, err
	}
	data, dataSig, err := chooseRecs(keys, resMsg)
	if err != nil {
		return nil, err
	}

	return &Result{
		Delegations: delegs,

		Keys: keys,
		Data: data,

		KeySig:  keySig,
		DataSig: dataSig,
	}, nil
}

func (r *Result) A(name string) ([]string, error) {
	name = strings.ToLower(dns.Fqdn(name))
	out := make([]string, 0)

	for _, rr := range r.Data {
		addr, ok := rr.(*dns.A)
		if !ok {
			return nil, fmt.Errorf("unexpected record type: %T", rr)
		} else if strings.ToLower(addr.Hdr.Name) != name {
			return nil, fmt.Errorf("unexpected record name: %v", addr.Hdr.Name)
		}
		out = append(out, fmt.Sprint(addr.A))
	}

	return out, nil
}

func (r *Result) AAAA(name string) ([]string, error) {
	name = strings.ToLower(dns.Fqdn(name))
	out := make([]string, 0)

	for _, rr := range r.Data {
		addr, ok := rr.(*dns.AAAA)
		if !ok {
			return nil, fmt.Errorf("unexpected record type: %T", rr)
		} else if strings.ToLower(addr.Hdr.Name) != name {
			return nil, fmt.Errorf("unexpected record name: %v", addr.Hdr.Name)
		}
		out = append(out, fmt.Sprint(addr.AAAA))
	}

	return out, nil
}

func (r *Result) TXT(name string) ([]string, error) {
	name = strings.ToLower(dns.Fqdn(name))
	out := make([]string, 0)

	for _, rr := range r.Data {
		txt, ok := rr.(*dns.TXT)
		if !ok {
			return nil, fmt.Errorf("unexpected record type: %T", rr)
		} else if strings.ToLower(txt.Hdr.Name) != name {
			return nil, fmt.Errorf("unexpected record name: %v", txt.Hdr.Name)
		}
		out = append(out, strings.Join(txt.Txt, ""))
	}

	return out, nil
}

func (r *Result) Verify() error {
	digests := rootDigests
	for _, deleg := range r.Delegations {
		if err := verifyKeyset(digests, deleg.Keys, deleg.KeySig); err != nil {
			return err
		}
		recs := make([]dns.RR, 0, len(deleg.Digests))
		for _, rr := range deleg.Digests {
			recs = append(recs, rr)
		}
		if err := verifyRecs(deleg.Keys, recs, deleg.DigestSig); err != nil {
			return err
		}
		digests = deleg.Digests
	}

	if err := verifyKeyset(digests, r.Keys, r.KeySig); err != nil {
		return err
	} else if err := verifyRecs(r.Keys, r.Data, r.DataSig); err != nil {
		return err
	}

	return nil
}

func (r *Result) MarshalBinary() ([]byte, error) {
	out := &pb.Result{}

	for _, del := range r.Delegations {
		raw, err := del.toPB()
		if err != nil {
			return nil, err
		}
		out.Delegations = append(out.Delegations, raw)
	}

	for _, key := range r.Keys {
		raw, err := packRR(key, r.KeySig)
		if err != nil {
			return nil, err
		}
		out.Keys = append(out.Keys, raw)
	}

	for _, data := range r.Data {
		raw, err := packRR(data, r.DataSig)
		if err != nil {
			return nil, err
		}
		out.Data = append(out.Data, raw)
	}

	keySig, err := packRR(r.KeySig, r.KeySig)
	if err != nil {
		return nil, err
	}
	out.KeySig = keySig

	dataSig, err := packRR(r.DataSig, r.DataSig)
	if err != nil {
		return nil, err
	}
	out.DataSig = dataSig

	return proto.Marshal(out)
}

// Delegation is evidence provided by one authority that they are delegating
// control of a zone to a lower authority. The lower authority may delegate
// again to an even lower authority, such that there's a chain of delegations
// starting at the root zone.
type Delegation struct {
	Keys    []*dns.DNSKEY
	Digests []*dns.DS

	KeySig, DigestSig *dns.RRSIG
}

func newDelegation(digests []*dns.DS, msgs delegMsg) (*Delegation, error) {
	keys, keySig, err := chooseKeyset(digests, msgs.keys)
	if err != nil {
		return nil, err
	}
	recs, digestSig, err := chooseRecs(keys, msgs.digests)
	if err != nil {
		return nil, err
	}

	ds := make([]*dns.DS, 0, len(recs))
	for _, rr := range recs {
		ds = append(ds, rr.(*dns.DS))
	}

	return &Delegation{
		Keys:    keys,
		Digests: ds,

		KeySig:    keySig,
		DigestSig: digestSig,
	}, nil
}

func (d Delegation) toPB() (*pb.Delegation, error) {
	out := &pb.Delegation{}

	for _, key := range d.Keys {
		raw, err := packRR(key, d.KeySig)
		if err != nil {
			return nil, err
		}
		out.Keys = append(out.Keys, raw)
	}

	for _, digest := range d.Digests {
		raw, err := packRR(digest, d.DigestSig)
		if err != nil {
			return nil, err
		}
		out.Digests = append(out.Digests, raw)
	}

	keySig, err := packRR(d.KeySig, d.KeySig)
	if err != nil {
		return nil, err
	}
	out.KeySig = keySig

	digestSig, err := packRR(d.DigestSig, d.DigestSig)
	if err != nil {
		return nil, err
	}
	out.DigestSig = digestSig

	return out, nil
}

func packRR(rr dns.RR, sig *dns.RRSIG) ([]byte, error) {
	// Do minimum sanitization that is necessary for the RRSIG to verify.
	hdr := rr.Header()
	hdr.Ttl = sig.OrigTtl

	labels := dns.SplitDomainName(hdr.Name)
	if len(labels) > int(sig.Labels) {
		hdr.Name = "*." + strings.Join(labels[len(labels)-int(sig.Labels):], ".") + "."
	}
	hdr.Name = strings.ToLower(hdr.Name)

	// Serialize RR.
	raw := make([]byte, len(hdr.Name)+int(hdr.Rdlength)+12)
	n, err := dns.PackRR(rr, raw, 0, nil, false)
	if err != nil {
		return nil, err
	}
	return raw[:n], nil
}
