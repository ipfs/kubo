package commands

import (
	"context"
	"errors"
	"strings"
	"time"

	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	loader "github.com/ipfs/kubo/plugin/loader"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
	config "github.com/ipfs/kubo/config"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
)

var log = logging.Logger("command")

// Context represents request context.
type Context struct {
	ConfigRoot string
	ReqLog     *ReqLog

	Plugins *loader.PluginLoader

	Gateway       bool
	api           coreiface.CoreAPI
	node          *core.IpfsNode
	ConstructNode func() (*core.IpfsNode, error)
}

func (c *Context) GetConfig() (*config.Config, error) {
	node, err := c.GetNode()
	if err != nil {
		return nil, err
	}
	return node.Repo.Config()
}

// GetNode returns the node of the current Command execution
// context. It may construct it with the provided function.
func (c *Context) GetNode() (*core.IpfsNode, error) {
	var err error
	if c.node == nil {
		if c.ConstructNode == nil {
			return nil, errors.New("nil ConstructNode function")
		}
		c.node, err = c.ConstructNode()
	}
	return c.node, err
}

// ClearCachedNode clears any cached node, forcing GetNode to construct a new one.
//
// This method is critical for mitigating racy FX dependency injection behavior
// that can occur during daemon startup. The daemon may create multiple IpfsNode
// instances during initialization - first an offline node during early init, then
// the proper online daemon node. Without clearing the cache, HTTP RPC handlers may
// end up using the first (offline) cached node instead of the intended online daemon node.
//
// This behavior was likely present forever in go-ipfs, but recent changes made it more
// prominent and forced us to proactively mitigate FX shortcomings. The daemon calls
// this method immediately before setting its ConstructNode function to ensure that
// subsequent GetNode() calls use the correct online daemon node rather than any
// stale cached offline node from initialization.
func (c *Context) ClearCachedNode() {
	c.node = nil
}

// GetAPI returns CoreAPI instance backed by ipfs node.
// It may construct the node with the provided function.
func (c *Context) GetAPI() (coreiface.CoreAPI, error) {
	if c.api == nil {
		n, err := c.GetNode()
		if err != nil {
			return nil, err
		}
		fetchBlocks := true
		if c.Gateway {
			cfg, err := c.GetConfig()
			if err != nil {
				return nil, err
			}
			fetchBlocks = !cfg.Gateway.NoFetch
		}

		c.api, err = coreapi.NewCoreAPI(n, options.Api.FetchBlocks(fetchBlocks))
		if err != nil {
			return nil, err
		}
	}
	return c.api, nil
}

// Context returns the node's context.
func (c *Context) Context() context.Context {
	n, err := c.GetNode()
	if err != nil {
		log.Debug("error getting node: ", err)
		return context.Background()
	}

	return n.Context()
}

// LogRequest adds the passed request to the request log and
// returns a function that should be called when the request
// lifetime is over.
func (c *Context) LogRequest(req *cmds.Request) func() {
	rle := &ReqLogEntry{
		StartTime: time.Now(),
		Active:    true,
		Command:   strings.Join(req.Path, "/"),
		Options:   req.Options,
		Args:      req.Arguments,
		log:       c.ReqLog,
	}
	c.ReqLog.AddEntry(rle)

	return func() {
		c.ReqLog.Finish(rle)
	}
}

// Close cleans up the application state.
func (c *Context) Close() {
	// let's not forget teardown. If a node was initialized, we must close it.
	// Note that this means the underlying req.Context().Node variable is exposed.
	// this is gross, and should be changed when we extract out the exec Context.
	if c.node != nil {
		log.Info("Shutting down node...")
		c.node.Close()
	}
}
