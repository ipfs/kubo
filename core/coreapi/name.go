package coreapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ipfs/boxo/ipns"
	keystore "github.com/ipfs/boxo/keystore"
	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/kubo/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs/boxo/path"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
	ci "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

type NameAPI CoreAPI

// Publish announces new IPNS name and returns the new IPNS entry.
func (api *NameAPI) Publish(ctx context.Context, p path.Path, opts ...caopts.NamePublishOption) (ipns.Name, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.NameAPI", "Publish", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	if err := api.checkPublishAllowed(); err != nil {
		return ipns.Name{}, err
	}

	options, err := caopts.NamePublishOptions(opts...)
	if err != nil {
		return ipns.Name{}, err
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
		return ipns.Name{}, err
	}

	k, err := keylookup(api.privateKey, api.repo.Keystore(), options.Key)
	if err != nil {
		return ipns.Name{}, err
	}

	eol := time.Now().Add(options.ValidTime)

	publishOptions := []namesys.PublishOption{
		namesys.PublishWithEOL(eol),
		namesys.PublishWithIPNSOption(ipns.WithV1Compatibility(options.CompatibleWithV1)),
	}

	if options.TTL != nil {
		publishOptions = append(publishOptions, namesys.PublishWithTTL(*options.TTL))
	}

	err = api.namesys.Publish(ctx, k, p, publishOptions...)
	if err != nil {
		return ipns.Name{}, err
	}

	pid, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return ipns.Name{}, err
	}

	return ipns.NameFromPeer(pid), nil
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

	p, err := path.NewPath(name)
	if err != nil {
		return nil, err
	}

	out := make(chan coreiface.IpnsResult)
	go func() {
		defer close(out)
		for res := range resolver.ResolveAsync(ctx, p, options.ResolveOpts...) {
			select {
			case out <- coreiface.IpnsResult{Path: res.Path, Err: res.Err}:
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

	return nil, errors.New("no key by the given name or PeerID was found")
}
