package coreapi

import (
	cid "github.com/ipfs/go-cid"
)

// ProviderAPI brings Provider behavior to CoreAPI
type ProviderAPI CoreAPI

// Provide the given cid using the current provider
func (api *ProviderAPI) Provide(cid cid.Cid) error {
	return api.provider.Provide(cid)
}
