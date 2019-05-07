package main

import (
	"context"
	"fmt"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	migrate "github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
)

type nodeBuilder struct {
	ctx      context.Context
	repoPath string

	migrate     bool
	userMigrate bool // --migrate flag present
	unencrypted bool

	routing string

	offline bool
	ipnsps  bool
	pubsub  bool
	mplex   bool

	permanent bool // only used for bloom filter in blockstore
	daemon bool
}

func (b *nodeBuilder) buildNode() (*core.IpfsNode, error) {
	if b.unencrypted {
		log.Errorf(`Running with --%s: All connections are UNENCRYPTED.
		You will not be able to connect to regular encrypted networks.`, unencryptTransportKwd)
	}

	// acquire the repo lock _before_ constructing a node. we need to make
	// sure we are permitted to access the resources (datastore, etc.)
	repo, err := fsrepo.Open(b.repoPath)
	switch err {
	default:
		return nil, err
	case fsrepo.ErrNeedMigration:
		fmt.Println("Found outdated fs-repo, migrations need to be run.")

		if !b.userMigrate {
			b.migrate = YesNoPrompt("Run migrations now? [y/N]")
		}

		if !b.migrate {
			fmt.Println("Not running migrations of fs-repo now.")
			fmt.Println("Please get fs-repo-migrations from https://dist.ipfs.io")
			return nil, fmt.Errorf("fs-repo requires migration")
		}

		err = migrate.RunMigration(fsrepo.RepoVersion)
		if err != nil {
			fmt.Println("The migrations of fs-repo failed:")
			fmt.Printf("  %s\n", err)
			fmt.Println("If you think this is a bug, please file an issue and include this whole log output.")
			fmt.Println("  https://github.com/ipfs/fs-repo-migrations")
			return nil, err
		}

		repo, err = fsrepo.Open(b.repoPath)
		if err != nil {
			return nil, err
		}
	case nil:
		break
	}

	var routing libp2p.RoutingOption
	routingOption := b.routing
	if routingOption == routingOptionDefaultKwd {
		cfg, err := repo.Config()
		if err != nil {
			repo.Close()
			return nil, err
		}

		routingOption = cfg.Routing.Type
		if routingOption == "" {
			routingOption = routingOptionDHTKwd
		}
	}
	switch routingOption {
	case routingOptionDHTClientKwd:
		routing = libp2p.DHTClientOption
	case routingOptionDHTKwd:
		routing = libp2p.DHTOption
	case routingOptionNoneKwd:
		routing = libp2p.NilRouterOption
	default:
		repo.Close()
		return nil, fmt.Errorf("unrecognized routing option: %s", routingOption)
	}

	// ok everything is good. set it on the invocation (for ownership)
	// and return it.
	node, err := core.NewNode(b.ctx, &core.BuildCfg{
		Repo: repo,

		DisableEncryptedConnections: b.unencrypted,
		Permanent: true, // It is temporary way to signify that node is permanent
		Online:    !b.offline,

		Routing: routing,

		ExtraOpts: map[string]bool{
			"pubsub": b.pubsub,
			"ipnsps": b.ipnsps,
			"mplex":  b.mplex,
		},
	})
	if err != nil {
		repo.Close()
		return nil, err
	}

	node.IsDaemon = b.daemon

	return node, nil
}
