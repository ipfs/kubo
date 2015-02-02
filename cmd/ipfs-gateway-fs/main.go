package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	core "github.com/jbenet/go-ipfs/core"
	corehttp "github.com/jbenet/go-ipfs/core/corehttp"
	corerepo "github.com/jbenet/go-ipfs/core/corerepo"
	coreunix "github.com/jbenet/go-ipfs/core/coreunix"
	config "github.com/jbenet/go-ipfs/repo/config"
	fsrepo "github.com/jbenet/go-ipfs/repo/fsrepo"
)

var (
	writable               = flag.Bool("writable", false, "enable writing objects (with POST, PUT and DELETE)")
	refreshAssetsInterval  = flag.Duration("refresh-assets-interval", 30*time.Second, "refresh assets")
	garbageCollectInterval = flag.Duration("gc-interval", 24*time.Hour, "frequency of repo garbage collection")
	assetsPath             = flag.String("assets-path", "", "if provided, periodically adds contents of path to IPFS")
	host                   = flag.String("host", "/ip4/0.0.0.0/tcp/8080", "override the HTTP host listening address")
	performGC              = flag.Bool("gc", false, "perform garbage collection")
	nBitsForKeypair        = flag.Int("b", 1024, "number of bits for keypair (if repo is uninitialized)")
)

func main() {
	flag.Parse()
	if *assetsPath == "" {
		log.Println("asset-path not provided. hosting gateway without file server functionality...")
	}
	if err := run(); err != nil {
		log.Println(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repoPath, err := fsrepo.BestKnownPath()
	if err != nil {
		return err
	}

	if !fsrepo.IsInitialized(repoPath) {
		conf, err := config.Init(os.Stdout, *nBitsForKeypair)
		if err != nil {
			return err
		}
		if err := fsrepo.Init(repoPath, conf); err != nil {
			return err
		}
	}

	repo := fsrepo.At(repoPath)
	if err := repo.Open(); err != nil { // owned by node
		return err
	}

	node, err := core.NewIPFSNode(ctx, core.Online(repo))
	if err != nil {
		return err
	}
	defer node.Close()

	if *performGC {
		if err := runGarbageCollectorWorker(ctx, node); err != nil {
			return err
		}
	}

	if *assetsPath != "" {
		if err := runFileServerWorker(ctx, node); err != nil {
			return err
		}
	}

	opts := []corehttp.ServeOption{
		corehttp.GatewayOption(*writable),
	}
	return corehttp.ListenAndServe(node, *host, opts...)
}

func runGarbageCollectorWorker(ctx context.Context, node *core.IpfsNode) error {
	go func() {
		for _ = range time.Tick(*garbageCollectInterval) {
			if err := corerepo.GarbageCollect(node, ctx); err != nil {
				log.Println("failed to run garbage collection", err)
			}
		}
	}()
	return nil
}

func runFileServerWorker(ctx context.Context, node *core.IpfsNode) error {
	fi, err := os.Stat(*assetsPath)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return errors.New("asset path must be a directory")
	}
	go func() {
		for _ = range time.Tick(*refreshAssetsInterval) {
			_, err := coreunix.AddR(node, *assetsPath)
			if err != nil {
				log.Println(err)
			}
		}
	}()
	return nil
}
