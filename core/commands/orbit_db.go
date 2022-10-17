package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	iface "github.com/ipfs/interface-go-ipfs-core"
	config "github.com/ipfs/kubo/config"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"

	orbitdb "berty.tech/go-orbit-db"
	"berty.tech/go-orbit-db/stores"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

type DBAddress struct {
	string
}

// const dbAddress = "/orbitdb/bafyreibe2lnmluj2y4byq6dnb4jbyxinvfbiz5lj7myasprln5pqtmcarm/demand_supply"

var OrbitCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "An experimental orbit-db integration on ipfs.",
		ShortDescription: `
orbit db is a p2p database on top of ipfs node
`,
	},
	Subcommands: map[string]*cmds.Command{
		"dbcreate": OrbitCreateCmd,
		"kvput":    OrbitPutCmd,
		"kvget":    OrbitGetCmd,
		"kvdel":    OrbitDelCmd,
	},
}

var OrbitCreateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create Orbit DB",
		ShortDescription: `Create the Orbit database
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("storeType", true, false, "Store type to be created"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		storeType := req.Arguments[0]

		switch storeType {
		case "keyval":
			dbAddress, err := CreateKeyValue(req.Context, api)
			if err != nil {
				return err
			}

			if err := res.Emit(&DBAddress{dbAddress}); err != nil {
				return err
			}
		case "docs":
			dbAddress, err := CreateDocs(req.Context, api)
			if err != nil {
				return err
			}

			if err := res.Emit(&DBAddress{dbAddress}); err != nil {
				return err
			}
		}

		return nil
	},
	Type: DBAddress{},
}

var OrbitPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Put value related to key",
		ShortDescription: `Key value store put
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "The database address"),
		cmds.StringArg("storeType", true, false, "Store type to be created"),
		cmds.StringArg("key", true, false, "Key"),
		cmds.FileArg("value", true, false, "Value").EnableStdin(),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbAddress := req.Arguments[0]
		storeType := req.Arguments[1]
		key := req.Arguments[2]

		// read data passed as a file
		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}

		switch storeType {
		case "keyval":
			db, store, err := ConnectKV(req.Context, dbAddress, api, func(address string) {})
			if err != nil {
				return err
			}

			defer db.Close()

			_, err = store.Put(req.Context, key, data)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

var OrbitGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get value by key",
		ShortDescription: `Key value store get
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "The database address"),
		cmds.StringArg("storeType", true, false, "Store type to be created"),
		cmds.StringArg("key", true, false, "Key to get related value"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbAddress := req.Arguments[0]
		storeType := req.Arguments[1]
		key := req.Arguments[2]

		switch storeType {
		case "keyval":
			db, store, err := ConnectKV(req.Context, dbAddress, api, func(address string) {})
			if err != nil {
				return err
			}

			defer db.Close()

			if key == "all" {
				val := store.All()
				if err != nil {
					return err
				}

				if err := res.Emit(&val); err != nil {
					return err
				}
			} else {
				val, err := store.Get(req.Context, key)
				if err != nil {
					return err
				}

				if err := res.Emit(&val); err != nil {
					return err
				}
			}
		}

		return nil
	},
	Type: []byte{},
}

var OrbitDelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Delete value by key",
		ShortDescription: `Key value store delete
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "The database address"),
		cmds.StringArg("storeType", true, false, "Store type to be created"),
		cmds.StringArg("key", true, false, "Key to delete related value"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbAddress := req.Arguments[0]
		storeType := req.Arguments[1]
		key := req.Arguments[2]

		switch storeType {
		case "keyval":
			db, store, err := ConnectKV(req.Context, dbAddress, api, func(address string) {})
			if err != nil {
				return err
			}

			defer db.Close()

			if key == "all" {
				val := store.All()
				if err != nil {
					return err
				}

				for i := range val {
					_, err := store.Delete(req.Context, i)
					if err != nil {
						return err
					}
				}
			} else {
				_, err := store.Delete(req.Context, key)
				if err != nil {
					return err
				}
			}
		}

		return nil
	},
}

func CreateKeyValue(ctx context.Context, api iface.CoreAPI) (string, error) {
	datastore := filepath.Join(os.Getenv("HOME"), ".ipfs", "orbitdb")
	if _, err := os.Stat(datastore); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(datastore), 0755)
	}

	db, err := orbitdb.NewOrbitDB(ctx, api, &orbitdb.NewOrbitDBOptions{
		Directory: &datastore,
	})
	if err != nil {
		return "", err
	}

	store, err := db.KeyValue(ctx, "", &orbitdb.CreateDBOptions{})
	if err != nil {
		return "", err
	}

	sub, err := db.EventBus().Subscribe(new(stores.EventReady))
	if err != nil {
		return "", err
	}

	defer sub.Close()

	err = connectToPeers(api, ctx)
	if err != nil {
		return "", err
	}

	var dbAddress string

	go func() {
		for {
			for ev := range sub.Out() {
				switch ev.(type) {
				case *stores.EventReady:
					dbAddress = store.Address().String()
				}
			}
		}
	}()

	return dbAddress, err
}

func CreateDocs(ctx context.Context, api iface.CoreAPI) (string, error) {
	datastore := filepath.Join(os.Getenv("HOME"), ".ipfs", "orbitdb")
	if _, err := os.Stat(datastore); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(datastore), 0755)
	}

	db, err := orbitdb.NewOrbitDB(ctx, api, &orbitdb.NewOrbitDBOptions{
		Directory: &datastore,
	})
	if err != nil {
		return "", err
	}

	store, err := db.Docs(ctx, "", &orbitdb.CreateDBOptions{})
	if err != nil {
		return "", err
	}

	sub, err := db.EventBus().Subscribe(new(stores.EventReady))
	if err != nil {
		return "", err
	}

	defer sub.Close()

	err = connectToPeers(api, ctx)
	if err != nil {
		return "", err
	}

	var dbAddress string

	go func() {
		for {
			for ev := range sub.Out() {
				switch ev.(type) {
				case *stores.EventReady:
					dbAddress = store.Address().String()
				}
			}
		}
	}()

	return dbAddress, err
}

func ConnectKV(ctx context.Context, dbAddress string, api iface.CoreAPI, onReady func(address string)) (orbitdb.OrbitDB, orbitdb.KeyValueStore, error) {
	datastore := filepath.Join(os.Getenv("HOME"), ".ipfs", "orbitdb")
	if _, err := os.Stat(datastore); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(datastore), 0755)
	}

	db, err := orbitdb.NewOrbitDB(ctx, api, &orbitdb.NewOrbitDBOptions{
		Directory: &datastore,
	})
	if err != nil {
		return db, nil, err
	}

	KvStore, err := db.KeyValue(ctx, dbAddress, &orbitdb.CreateDBOptions{})
	if err != nil {
		return db, nil, err
	}

	// for remote db only
	// deprecated
	// evs := KvStore.Subscribe(ctx)

	sub, err := db.EventBus().Subscribe(new(stores.EventReady))
	if err != nil {
		return db, nil, err
	}

	defer sub.Close()

	err = connectToPeers(api, ctx)
	if err != nil {
		return db, nil, err
	}

	go func() {
		for {
			for ev := range sub.Out() {
				switch ev.(type) {
				case *stores.EventReady:
					dbURI := KvStore.Address().String()
					onReady(dbURI)
				}
			}
		}
	}()

	KvStore.Load(ctx, -1)
	if err != nil {
		return db, nil, err
	}

	return db, KvStore, nil
}

func connectToPeers(c iface.CoreAPI, ctx context.Context) error {
	var wg sync.WaitGroup

	peerInfos, err := config.DefaultBootstrapPeers()
	if err != nil {
		return err
	}

	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peer.AddrInfo) {
			defer wg.Done()
			err := c.Swarm().Connect(ctx, *peerInfo)
			if err != nil {
				fmt.Println("failed to connect", zap.String("peerID", peerInfo.ID.String()), zap.Error(err))
			} else {
				fmt.Println("connected!", zap.String("peerID", peerInfo.ID.String()))
			}
		}(&peerInfo)
	}
	wg.Wait()
	return nil
}
