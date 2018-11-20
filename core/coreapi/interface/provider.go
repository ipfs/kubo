package iface

import "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

type ProviderAPI interface {
	// Announce that the given cid is being provided
	Provide(cid.Cid)
}

