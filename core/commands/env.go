package commands

import (
	"fmt"

	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	cmds "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	config "gx/ipfs/QmTyiSs9VgdVb4pnzdjtKhcfdTkHFEaNn6xnCbZq4DTFRt/go-ipfs-config"
)

// GetNode extracts the node from the environment.
func GetNode(env interface{}) (*core.IpfsNode, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetNode()
}

// GetApi extracts CoreAPI instance from the environment.
func GetApi(env cmds.Environment) (coreiface.CoreAPI, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetApi()
}

// GetConfig extracts the config from the environment.
func GetConfig(env cmds.Environment) (*config.Config, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetConfig()
}
