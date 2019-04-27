package client

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

const (
	DefaultPathName = ".ipfs"
	DefaultPathRoot = "~/" + DefaultPathName
	EnvDir          = "IPFS_PATH"
)

func repo() (string, error) {
	baseDir := os.Getenv(EnvDir)
	if baseDir == "" {
		baseDir = DefaultPathRoot
	}
	return homedir.Expand(baseDir)
}

//////////////////////

type ManagedApi interface {
	iface.CoreAPI

	io.Closer
}

type ctxApi struct {
	iface.CoreAPI
	context.CancelFunc
}

func (a *ctxApi) Close() error {
	a.CancelFunc()
	return nil
}

var _ ManagedApi = &ctxApi{}

//////////////////////

func embedded(repoPath string) (ManagedApi, error) {
	ctx, cancel := context.WithCancel(context.TODO())

	r, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening fsrepo failed: %s", err)
	}
	n, err := core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		Repo:   r,
	})
	if err != nil {
		return nil, fmt.Errorf("ipfs NewNode() failed: %s", err)
	}

	api, err := coreapi.NewCoreAPI(n)
	if err != nil {
		return nil, err
	}

	return &ctxApi{
		CoreAPI: api,
		CancelFunc: cancel,
	}, nil
}

type ApiProvider func() (ManagedApi, error)

var localApi ApiProvider = func() (ManagedApi, error) {
	api, err := httpapi.NewLocalApi()
	if err != nil {
		return nil, err
	}

	return &ctxApi{
		CoreAPI: api,
		CancelFunc: func() {},
	}, nil
}

var tempEmbedded ApiProvider = func() (ManagedApi, error) {
	dir, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return nil, fmt.Errorf("failed to get temp dir: %s", err)
	}

	cfg, err := config.Init(ioutil.Discard, 1024)
	if err != nil {
		return nil, err
	}

	err = fsrepo.Init(dir, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return embedded(dir)
}

var localEmbedded ApiProvider = func() (ManagedApi, error) {
	repoPath, err := repo()
	if err != nil {
		return nil, err
	}

	return embedded(repoPath)
}

// TODO: socket activation? 'manual' daemon spawning?
// TODO: repo (config?) option to tell apps to not spawn embedded nodes

// New creates new api client
func New() (iface.CoreAPI, error) {
	api, err := localApi()
	if api != nil || err != nil {
		return api, err
	}

	api, err = localEmbedded()
	if api != nil || err != nil {
		return api, err
	}

	api, err = tempEmbedded()
	if api != nil || err != nil {
		return api, err
	}

	return nil, errors.New("failed to create a node")
}
