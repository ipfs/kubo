package coreapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	keystore "github.com/ipfs/go-ipfs-keystore"
	"github.com/ipfs/go-namesys"
	"github.com/ipfs/kubo/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	ipath "github.com/ipfs/go-path"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	nsopts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	ci "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

type NameAPI CoreAPI

type ipnsEntry struct {
	name  string
	value path.Path
}

// Name returns the ipnsEntry name.
func (e *ipnsEntry) Name() string {
	return e.name
}

// Value returns the ipnsEntry value.
func (e *ipnsEntry) Value() path.Path {
	return e.value
}

// Publish announces new IPNS name and returns the new IPNS entry.
func (api *NameAPI) Publish(ctx context.Context, p path.Path, opts ...caopts.NamePublishOption) (coreiface.IpnsEntry, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.NameAPI", "Publish", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	if err := api.checkPublishAllowed(); err != nil {
		return nil, err
	}

	options, err := caopts.NamePublishOptions(opts...)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(
		attribute.Bool("allowoffline", options.AllowOffline),
		attribute.String("key", options.Key),
		attribute.Float64("validtime", options.ValidTime.Seconds()),
	)
	if options.TTL != nil {
		span.SetAttributes(attribute.Float64("ttl", options.TTL.Seconds()))
	}

	err = api.checkOnline(options.AllowOffline)
	if err != nil {
		return nil, err
	}

	pth, err := ipath.ParsePath(p.String())
	if err != nil {
		return nil, err
	}

	k, err := keylookup(api.privateKey, api.repo.Keystore(), options.Key)
	if err != nil {
		return nil, err
	}

	eol := time.Now().Add(options.ValidTime)

	publishOptions := []nsopts.PublishOption{
		nsopts.PublishWithEOL(eol),
	}

	if options.TTL != nil {
		publishOptions = append(publishOptions, nsopts.PublishWithTTL(*options.TTL))
	}

	err = api.namesys.Publish(ctx, k, pth, publishOptions...)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return nil, err
	}

	return &ipnsEntry{
		name:  coreiface.FormatKeyID(pid),
		value: p,
	}, nil
}

func (api *NameAPI) Search(ctx context.Context, name string, opts ...caopts.NameResolveOption) (<-chan coreiface.IpnsResult, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.NameAPI", "Search", trace.WithAttributes(attribute.String("name", name)))
	defer span.End()

	options, err := caopts.NameResolveOptions(opts...)
	if err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.Bool("cache", options.Cache))

	err = api.checkOnline(true)
	if err != nil {
		return nil, err
	}

	var resolver namesys.Resolver = api.namesys
	if !options.Cache {
		resolver, err = namesys.NewNameSystem(api.routing,
			namesys.WithDatastore(api.repo.Datastore()),
			namesys.WithDNSResolver(api.dnsResolver))
		if err != nil {
			return nil, err
		}
	}

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	out := make(chan coreiface.IpnsResult)
	go func() {
		defer close(out)
		for res := range resolver.ResolveAsync(ctx, name, options.ResolveOpts...) {
			select {
			case out <- coreiface.IpnsResult{Path: path.New(res.Path.String()), Err: res.Err}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// Resolve attempts to resolve the newest version of the specified name and
// returns its path.
func (api *NameAPI) Resolve(ctx context.Context, name string, opts ...caopts.NameResolveOption) (path.Path, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.NameAPI", "Resolve", trace.WithAttributes(attribute.String("name", name)))
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results, err := api.Search(ctx, name, opts...)
	if err != nil {
		return nil, err
	}

	err = coreiface.ErrResolveFailed
	var p path.Path

	for res := range results {
		p, err = res.Path, res.Err
		if err != nil {
			break
		}
	}

	return p, err
}

func keylookup(self ci.PrivKey, kstore keystore.Keystore, k string) (ci.PrivKey, error) {
	////////////////////
	// Lookup by name //
	////////////////////

	// First, lookup self.
	if k == "self" {
		return self, nil
	}

	// Then, look in the keystore.
	res, err := kstore.Get(k)
	if res != nil {
		return res, nil
	}

	if err != nil && err != keystore.ErrNoSuchKey {
		return nil, err
	}

	keys, err := kstore.List()
	if err != nil {
		return nil, err
	}

	//////////////////
	// Lookup by ID //
	//////////////////
	targetPid, err := peer.Decode(k)
	if err != nil {
		return nil, keystore.ErrNoSuchKey
	}

	// First, check self.
	pid, err := peer.IDFromPrivateKey(self)
	if err != nil {
		return nil, fmt.Errorf("failed to determine peer ID for private key: %w", err)
	}
	if pid == targetPid {
		return self, nil
	}

	// Then, look in the keystore.
	for _, key := range keys {
		privKey, err := kstore.Get(key)
		if err != nil {
			return nil, err
		}

		pid, err := peer.IDFromPrivateKey(privKey)
		if err != nil {
			return nil, err
		}

		if targetPid == pid {
			return privKey, nil
		}
	}

	return nil, fmt.Errorf("no key by the given name or PeerID was found")
}
