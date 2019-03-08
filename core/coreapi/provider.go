package coreapi

import (
	cid "github.com/ipfs/go-cid"
)

type ProviderAPI CoreAPI

func (api *ProviderAPI) Provide(root cid.Cid) error {
	return api.provider.Provide(root)
}
