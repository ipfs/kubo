package provider

import (
	"context"
	"github.com/ipfs/go-cid"
)

// ProviderSystem bundles together provider and reprovider behavior
// into one system
type ProviderSystem struct {
	provider   Provider
	reprovider *Reprovider
}

// NewProviderSystem creates a new ProviderSystem
func NewProviderSystem(p Provider, r *Reprovider) *ProviderSystem {
	return &ProviderSystem{
		provider:   p,
		reprovider: r,
	}
}

// Run starts the provider system loops
func (ps *ProviderSystem) Run() {
	ps.provider.Run()
	ps.reprovider.Run()
}

// Close stops the provider system loops
func (ps *ProviderSystem) Close() error {
	var errs []error

	if err := ps.provider.Close(); err != nil {
		errs = append(errs, err)
	}

	if err := ps.reprovider.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Provide a cid by announcing it to the network
func (ps *ProviderSystem) Provide(cid cid.Cid) {
	ps.provider.Provide(cid)
}

// Tracking returns all cids that are currently being tracked and reprovided
// by the provider system.
func (ps *ProviderSystem) Tracking() (<-chan cid.Cid, error) {
	return ps.provider.Tracking()
}

// Reprovide triggers a reprovide
func (ps *ProviderSystem) Reprovide(ctx context.Context) error {
	return ps.reprovider.Trigger(ctx)
}
