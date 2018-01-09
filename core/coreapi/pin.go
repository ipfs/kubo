package coreapi

import (
	"context"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	"github.com/pkg/errors"
)

type PinAPI struct {
	*CoreAPI
	*caopts.PinOptions
}

func (api *PinAPI) Add(context.Context, coreiface.Path, ...caopts.PinAddOption) error {
	return errors.New("TODO")
}

func (api *PinAPI) Ls(context.Context) ([]coreiface.Pin, error) {
	return nil, errors.New("TODO")
}

func (api *PinAPI) Rm(context.Context, coreiface.Path) error {
	return errors.New("TODO")
}

func (api *PinAPI) Update(ctx context.Context, from coreiface.Path, to coreiface.Path) error {
	return errors.New("TODO")
}

func (api *PinAPI) Verify(context.Context) error {
	return errors.New("TODO")
}
