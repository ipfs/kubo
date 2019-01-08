package httpapi

import (
	"context"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"github.com/libp2p/go-libp2p-peer"
)

type KeyAPI HttpApi

type keyOutput struct {
	JName string `json:"Name"`
	Id   string
}

func (k *keyOutput) Name() string {
	return k.JName
}

func (k *keyOutput) Path() iface.Path {
	p, _ := iface.ParsePath("/ipns/" + k.Id)
	return p
}

func (k *keyOutput) ID() peer.ID {
	p, _ := peer.IDB58Decode(k.Id)
	return p
}

func (k *keyOutput) valid() error {
	_, err := peer.IDB58Decode(k.Id)
	return err
}


func (api *KeyAPI) Generate(ctx context.Context, name string, opts ...caopts.KeyGenerateOption) (iface.Key, error) {
	options, err := caopts.KeyGenerateOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out keyOutput
	err = api.core().request("key/gen", name).
		Option("type", options.Algorithm).
		Option("size", options.Size).
		Exec(ctx, &out)
	if err != nil {
		return nil, err
	}
	if err := out.valid(); err != nil {
		return nil, err
	}
	return &out, nil
}

func (api *KeyAPI) Rename(ctx context.Context, oldName string, newName string, opts ...caopts.KeyRenameOption) (iface.Key, bool, error) {
	panic("implement me")
}

func (api *KeyAPI) List(ctx context.Context) ([]iface.Key, error) {
	panic("implement me")
}

func (api *KeyAPI) Self(ctx context.Context) (iface.Key, error) {
	var id struct{ID string}
	if err := api.core().request("id").Exec(ctx, &id); err != nil {
		return nil, err
	}

	out := keyOutput{JName: "self", Id: id.ID}
	if err := out.valid(); err != nil {
		return nil, err
	}
	return &out, nil
}

func (api *KeyAPI) Remove(ctx context.Context, name string) (iface.Key, error) {
	panic("implement me")
}

func (api *KeyAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
