package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/keystore"
	"github.com/ipfs/go-ipfs/repo"
	"syscall/js"
	"time"

	ci "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	"gx/ipfs/QmPJxxDsX2UbchSHobbYuvz7qnyJTFKvaKMzE2rZWJ4x5B/go-libp2p-peer"
	pstore "gx/ipfs/QmQFFp4ntkd4C14sP3FaH9WJyBuetuGUVo6dShNHvnoEvC/go-libp2p-peerstore"
	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	"gx/ipfs/QmTbcMKv6GU3fxhnNcbzYChdox9Fdd7VpucM3PQ7UWjX3D/go-ipfs-config"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	"gx/ipfs/QmebEmt23jQxrwnqBkFL4qbpE8EnnQunpv5U32LS5ESus1/go-libp2p"
	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
	syncds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore/sync"
	p2phost "gx/ipfs/QmfRHxh8bt4jWLKRhNvR5fn7mFACrQBFLqV4wyoymEExKV/go-libp2p-host"
)

func constructPeerHost(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (p2phost.Host, error) {
	pkey := ps.PrivKey(id)
	if pkey == nil {
		return nil, fmt.Errorf("missing private key for node ID: %s", id.Pretty())
	}
	options = append(options,
		libp2p.Identity(pkey),
		libp2p.NoTransports,
		libp2p.Transport(NewJsWs),
		libp2p.Peerstore(ps),
		libp2p.EnableAutoRelay(),
		libp2p.EnableRelay())
	return libp2p.New(ctx, options...)
}

func main() {
	logging.SetDebugLogging()
	ctx := context.Background()

	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 2048)
	if err != nil {
		panic(err)
	}

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		panic(err)
	}

	kbytes, err := sk.Bytes()
	if err != nil {
		panic(err)
	}

	ident := config.Identity{
		PeerID:  id.Pretty(),
		PrivKey: base64.StdEncoding.EncodeToString(kbytes),
	}

	c := config.Config{
		Bootstrap: []string{
			"/ip4/127.0.0.1/tcp/4006/ws/ipfs/QmZatNPNW8DnpMRgSuUJjmzc6nETJi21c2quikh4jbmPKk",
			"/ip4/51.75.35.194/tcp/4002/ws/ipfs/QmVGX47BzePPqEzpkTwfUJogPZxHcifpSXsGdgyHjtk5t7",
		},
	}
	c.Identity = ident
	c.Swarm.DisableNatPortMap = true

	r := &repo.Mock{
		C: c,
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
		K: keystore.NewMemKeystore(),
	}

	ncfg := &core.BuildCfg{
		Repo:                        r,
		Permanent:                   true, // It is temporary way to signify that node is permanent
		Online:                      true,
		DisableEncryptedConnections: false,
		Host:                        constructPeerHost,
		ExtraOpts: map[string]bool{
			"pubsub": false,
			"ipnsps": false,
			"mplex":  false,
		},
	}

	browserNode, err := core.NewNode(ctx, ncfg)
	if err != nil {
		panic(err)
	}

	api, err := coreapi.NewCoreAPI(browserNode)
	if err != nil {
		panic(err)
	}

	p, err := api.Unixfs().Add(ctx, files.NewBytesFile([]byte("an unique string not to be found anywhere else")))
	if err != nil {
		panic(err)
	}

	println(p.String())

	js.Global().Get("document").Call("getElementById", "addForm").Set("onsubmit", js.NewCallback(func(args []js.Value) {
		s := js.Global().Get("document").Call("getElementById", "toadd").Get("value").String() + "\n"
		p, err := api.Unixfs().Add(ctx, files.NewBytesFile([]byte(s)))
		if err != nil {
			panic(err)
		}

		pe := js.Global().Get("document").Call("createElement", "p")
		pe.Call("appendChild", js.Global().Get("document").Call("createTextNode", p.String()))
		results := js.Global().Get("document").Call("getElementById", "results")
		results.Call("insertBefore", pe, results.Get("firstChild"))
	}))

	t := time.Tick(time.Second)
	for range t {
		peers := js.Global().Get("document").Call("getElementById", "peers")
		peers.Set("innerHTML", "")
		list, err := api.Swarm().Peers(ctx)
		if err != nil {
			panic(err)
		}

		for _, ci := range list {
			pe := js.Global().Get("document").Call("createElement", "p")
			pe.Call("appendChild", js.Global().Get("document").Call("createTextNode", ci.Address().String()))
			peers.Call("appendChild", pe)
		}
	}

	wch := make(chan struct{})
	<-wch
}
