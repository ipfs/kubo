package provider

import "github.com/ipfs/go-cid"

type ProviderSystem struct {
	provider Provider
}

func NewProviderSystem(p Provider) *ProviderSystem {
	return &ProviderSystem{
		provider: p,
	}
}

func (ps *ProviderSystem) Run() {
	ps.provider.Run()
}

func (ps *ProviderSystem) Close() error {
	return ps.provider.Close()
}

func (ps *ProviderSystem) Provide(cid cid.Cid) {
	ps.provider.Provide(cid)
}

func (ps *ProviderSystem) Tracking() (<-chan cid.Cid, error) {
	return ps.provider.Tracking()
}
