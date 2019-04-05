package provider

import "github.com/ipfs/go-cid"

type offlineProvider struct{}

// NewOfflineProvider creates a Provider that does nothing
func NewOfflineProvider() Provider {
	return &offlineProvider{}
}

func (op *offlineProvider) Run() {}

func (op *offlineProvider) Provide(cid cid.Cid) error {
	return nil
}

func (op *offlineProvider) Close() error {
	return nil
}
