package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	gopath "path"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	random "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-random"
	"github.com/jbenet/go-ipfs/util/ipfsaddr"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	commands "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	corehttp "github.com/jbenet/go-ipfs/core/corehttp"
	corerouting "github.com/jbenet/go-ipfs/core/corerouting"
	"github.com/jbenet/go-ipfs/core/coreunix"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	"github.com/jbenet/go-ipfs/repo"
	config "github.com/jbenet/go-ipfs/repo/config"
	fsrepo "github.com/jbenet/go-ipfs/repo/fsrepo"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	unit "github.com/jbenet/go-ipfs/thirdparty/unit"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
)

var elog = eventlog.Logger("gc-client")

var (
	cat             = flag.Bool("cat", false, "else add")
	seed            = flag.Int64("seed", 1, "")
	nBitsForKeypair = flag.Int("b", 1024, "number of bits for keypair (if repo is uninitialized)")
)

var bootstrapAddresses = []string{
	"/ip4/104.236.70.34/tcp/4001/ipfs/QmaWJw5mcWkCThPtC7hVq28e3WbwLHnWF8HbMNJrRDybE4",
	"/ip4/128.199.72.111/tcp/4001/ipfs/Qmd2cSiZUt7vhiuTmqBB7XWbkuFR8KMLiEuQssjyNXyaZT",
	"/ip4/162.243.251.152/tcp/4001/ipfs/QmeaDCMmFWDuF4baxhMuvQH8APoymtMSJwhyZqUp9ux3SN",
	"/ip4/178.62.8.190/tcp/4001/ipfs/QmdXzZ25cyzSF99csCQmmPZ1NTbWTe8qtKFaZKpZQPdTFB",
	"/ip4/188.166.8.195/tcp/4001/ipfs/QmNU1Vpryj5hfSmybSYHnS497ttgy9aNJ3T2B8wY2uMso4",
	"/ip4/188.226.225.73/tcp/4001/ipfs/QmYHGLxLfHFd9veUt159LMhLD8LqhDjBUS9ZehLkPap6od",
	"/ip4/192.241.209.121/tcp/4001/ipfs/QmNPM851fxWb3qjLV66a8Nqtw7fBnMw2UrdFQidSdBQnvS",
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("using gcr remotes:")
	for _, p := range bootstrapAddresses {
		fmt.Println("\t", p)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoPath := gopath.Join(cwd, ".go-ipfs")
	if err := ensureRepoInitialized(repoPath); err != nil {
	}
	repo := fsrepo.At(repoPath)
	if err := repo.Open(); err != nil { // owned by node
		return err
	}
	cfg := repo.Config()
	cfg.Bootstrap = bootstrapAddresses
	if err := repo.SetConfig(cfg); err != nil {
		return err
	}

	var addrs []ipfsaddr.IPFSAddr
	for _, info := range bootstrapAddresses {
		addr, err := ipfsaddr.ParseString(info)
		if err != nil {
			return err
		}
		addrs = append(addrs, addr)
	}

	node, err := core.NewIPFSNode(ctx, core.OnlineWithRouting(repo, corerouting.SupernodeClient(addrs...)))
	if err != nil {
		return err
	}
	defer node.Close()

	opts := []corehttp.ServeOption{
		corehttp.CommandsOption(cmdCtx(node, repoPath)),
		corehttp.GatewayOption(false),
	}

	if *cat {
		if err := runFileCattingWorker(ctx, node); err != nil {
			return err
		}
	} else {
		if err := runFileAddingWorker(node); err != nil {
			return err
		}
	}
	return corehttp.ListenAndServe(node, cfg.Addresses.API, opts...)
}

func ensureRepoInitialized(path string) error {
	if !fsrepo.IsInitialized(path) {
		conf, err := config.Init(ioutil.Discard, *nBitsForKeypair)
		if err != nil {
			return err
		}
		if err := fsrepo.Init(path, conf); err != nil {
			return err
		}
	}
	return nil
}

func sizeOfIthFile(i int64) int64 {
	return (1 << uint64(i)) * unit.KB
}

func runFileAddingWorker(n *core.IpfsNode) error {
	go func() {
		var i int64
		for i = 1; i < math.MaxInt32; i++ {
			piper, pipew := io.Pipe()
			go func() {
				defer pipew.Close()
				if err := random.WritePseudoRandomBytes(sizeOfIthFile(i), pipew, *seed); err != nil {
					log.Fatal(err)
				}
			}()
			k, err := coreunix.Add(n, piper)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("added file", "seed", *seed, "#", i, "key", k, "size", unit.Information(sizeOfIthFile(i)))
			time.Sleep(1 * time.Second)
		}
	}()
	return nil
}

func runFileCattingWorker(ctx context.Context, n *core.IpfsNode) error {
	conf, err := config.Init(ioutil.Discard, *nBitsForKeypair)
	if err != nil {
		return err
	}

	dummy, err := core.NewIPFSNode(ctx, core.Offline(&repo.Mock{
		D: ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore())),
		C: *conf,
	}))
	if err != nil {
		return err
	}

	go func() {
		defer dummy.Close()
		var i int64 = 1
		for {
			var buf bytes.Buffer
			if err := random.WritePseudoRandomBytes(sizeOfIthFile(i), &buf, *seed); err != nil {
				log.Fatal(err)
			}
			// add to a dummy node to discover the key
			k, err := coreunix.Add(dummy, bytes.NewReader(buf.Bytes()))
			if err != nil {
				log.Fatal(err)
			}
			e := elog.EventBegin(ctx, "cat", eventlog.LoggableF(func() map[string]interface{} {
				return map[string]interface{}{
					"key":       k,
					"localPeer": n.Identity,
				}
			}))
			if r, err := coreunix.Cat(n, k); err != nil {
				e.Done()
				log.Printf("failed to cat file. seed: %d #%d key: %s err: %s", *seed, i, k, err)
			} else {
				log.Println("found file", "seed", *seed, "#", i, "key", k, "size", unit.Information(sizeOfIthFile(i)))
				io.Copy(ioutil.Discard, r)
				e.Done()
				log.Println("catted file", "seed", *seed, "#", i, "key", k, "size", unit.Information(sizeOfIthFile(i)))
				i++
			}
			time.Sleep(time.Second)
		}
	}()
	return nil
}

func toPeerInfos(bpeers []config.BootstrapPeer) ([]peer.PeerInfo, error) {
	var peers []peer.PeerInfo
	for _, bootstrap := range bpeers {
		p, err := toPeerInfo(bootstrap)
		if err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}
	return peers, nil
}

func toPeerInfo(bootstrap config.BootstrapPeer) (p peer.PeerInfo, err error) {
	p = peer.PeerInfo{
		ID:    bootstrap.ID(),
		Addrs: []ma.Multiaddr{bootstrap.Multiaddr()},
	}
	return p, nil
}

func cmdCtx(node *core.IpfsNode, repoPath string) commands.Context {
	return commands.Context{
		// TODO deprecate this shit
		Context:    context.Background(),
		Online:     true,
		ConfigRoot: repoPath,
		LoadConfig: func(path string) (*config.Config, error) {
			return node.Repo.Config(), nil
		},
		ConstructNode: func() (*core.IpfsNode, error) {
			return node, nil
		},
	}
}
