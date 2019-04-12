package provider

import (
	"context"
	"github.com/ipfs/go-cid"
)

// System bundles together provider and reprovider behavior
// into one system
type System struct {
	provider   Provider
	reprovider *Reprovider
}

// NewSystem creates a new System
func NewSystem(p Provider, r *Reprovider) *System {
	return &System{
		provider:   p,
		reprovider: r,
	}
}

// Run starts the provider system loops
func (ps *System) Run() {
	ps.provider.Run()
	ps.reprovider.Run()
}

// Close stops the provider system loops
func (ps *System) Close() error {
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
func (ps *System) Provide(cid cid.Cid) {
	ps.provider.Provide(cid)
}

// Tracking returns all cids that are currently being tracked and reprovided
// by the provider system.
func (ps *System) Tracking() (<-chan cid.Cid, error) {
	return ps.provider.Tracking()
}

// Reprovide triggers a reprovide
func (ps *System) Reprovide(ctx context.Context) error {
	return ps.reprovider.Reprovide(ctx)
}
