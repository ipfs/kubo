package provider

import (
	"context"
	"github.com/ipfs/go-cid"
)

type ProviderSystem struct {
	provider Provider
	reprovider *Reprovider
}

func NewProviderSystem(p Provider, r *Reprovider) *ProviderSystem {
	return &ProviderSystem{
		provider: p,
		reprovider: r,
	}
}

func (ps *ProviderSystem) Run() {
	ps.provider.Run()
	ps.reprovider.Run()
}

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

func (ps *ProviderSystem) Provide(cid cid.Cid) {
	ps.provider.Provide(cid)
}

func (ps *ProviderSystem) Tracking() (<-chan cid.Cid, error) {
	return ps.provider.Tracking()
}

func (ps *ProviderSystem) Reprovide(ctx context.Context) error {
    return ps.reprovider.Trigger(ctx)
}
