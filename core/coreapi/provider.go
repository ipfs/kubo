package coreapi

import "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

type ProviderAPI CoreAPI

func (api *ProviderAPI) Provide(cid cid.Cid) {
	api.node.Provider.Provide(cid)
}

