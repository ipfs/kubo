package provider

import "github.com/ipfs/go-cid"

type offlineProvider struct {}

func NewOfflineProvider() Provider {
	return &offlineProvider{}
}

func (op *offlineProvider) Run() {}

func (op *offlineProvider) Provide(cid cid.Cid) error {
	return nil
}
