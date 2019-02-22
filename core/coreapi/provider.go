package coreapi

import (
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
)

type ProviderAPI CoreAPI

func (api *ProviderAPI) Provide(root cid.Cid) error {
	return api.provider.Provide(root)
}

func (api *ProviderAPI) Unprovide(root cid.Cid) error {
	return api.provider.Unprovide(root)
}
