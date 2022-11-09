package commands

import (
	"context"
	"encoding/json"
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
	orbitdb_iface "berty.tech/go-orbit-db/iface"
	"berty.tech/go-orbit-db/stores"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

const dbAddressDemandSupply = "/orbitdb/bafyreia6t57n2uyfgpwpjqsztoxiiluc5xwcidfrfulc4i2quyd65uhmpe/demand_supply"
const dbAddressCitizenReputation = "/orbitdb/bafyreide5xex6dwtdg45eserwx2ib2cjeqpfu4hcjnik26hzvl525rwqoy/citizen_reputation"
const dbAddressIssue = "/orbitdb/bafyreihjgftbxrabuhfjn7diwtlz67hnhu2jsivds5arhqqb3xybov7eku/issue"
const dbAddressEvent = "/orbitdb/bafyreifhynz6quosu65iszr46b6jw3qlfdpgbvqwincnqc72hvhwkan3bm/event"

var OrbitCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "An experimental orbit-db integration on ipfs.",
		ShortDescription: `
orbit db is a p2p database on top of ipfs node
`,
	},
	Subcommands: map[string]*cmds.Command{
		"kvput":     OrbitPutKVCmd,
		"kvget":     OrbitGetKVCmd,
		"kvdel":     OrbitDelKVCmd,
		"docsput":   OrbitPutDocsCmd,
		"docsget":   OrbitGetDocsCmd,
		"docsquery": OrbitQueryDocsCmd,
		"docsdel":   OrbitDelDocsCmd,
	},
}

var OrbitPutKVCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Put value related to key",
		ShortDescription: `Key value store put
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key"),
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
		key := req.Arguments[1]

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

		db, store, err := ConnectKV(req.Context, dbAddress, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		_, err = store.Put(req.Context, key, data)
		if err != nil {
			return err
		}

		return nil
	},
}

var OrbitGetKVCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get value by key",
		ShortDescription: `Key value store get
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "DB address or name"),
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
		key := req.Arguments[1]

		db, store, err := ConnectKV(req.Context, dbAddress, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			val := store.All()

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

		return nil
	},
	Type: []byte{},
}

var OrbitDelKVCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Delete value by key",
		ShortDescription: `Key value store delete
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "DB address or name"),
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
		key := req.Arguments[1]

		db, store, err := ConnectKV(req.Context, dbAddress, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			val := store.All()

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

		return nil
	},
}

var OrbitPutDocsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Put value related to key",
		ShortDescription: `Key value store put
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "DB address or name"),
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

		dd := make(map[string]interface{})
		if err := json.Unmarshal([]byte(data), &dd); err != nil {
			panic(err)
		}

		dbAddress := req.Arguments[0]

		db, store, err := ConnectDocs(req.Context, dbAddress, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		_, err = store.Put(req.Context, dd)
		if err != nil {
			return err
		}

		return nil
	},
}

var OrbitGetDocsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get value by key",
		ShortDescription: `Key value store get
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "DB address or name"),
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
		key := req.Arguments[1]

		db, store, err := ConnectDocs(req.Context, dbAddress, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			val, err := store.Get(req.Context, "", &orbitdb_iface.DocumentStoreGetOptions{CaseInsensitive: false})
			if err != nil {
				return err
			}

			if err := res.Emit(&val); err != nil {
				return err
			}
		} else {
			val, err := store.Get(req.Context, key, &orbitdb_iface.DocumentStoreGetOptions{CaseInsensitive: false})
			if err != nil {
				return err
			}

			if err := res.Emit(&val[0]); err != nil {
				return err
			}
		}

		return nil
	},
	Type: []byte{},
}

var OrbitQueryDocsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Query docs by key and value",
		ShortDescription: `Query docs store by key and value
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key to query"),
		cmds.StringArg("query", true, false, "Value to query"),
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
		key := req.Arguments[1]
		query := req.Arguments[2]

		db, store, err := ConnectDocs(req.Context, dbAddress, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		q, err := store.Query(req.Context, func(e interface{}) (bool, error) {
			issue := e.(map[string]interface{})
			if issue[key] == query {
				return true, nil
			}
			return false, nil
		})

		if err != nil {
			return err
		}

		if err := res.Emit(&q); err != nil {
			return err
		}

		return nil
	},
	Type: []byte{},
}

var OrbitDelDocsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Delete value by key",
		ShortDescription: `Key value store delete
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbAddress", true, false, "DB address or name"),
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
		key := req.Arguments[1]

		db, store, err := ConnectDocs(req.Context, dbAddress, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			var issues []map[string]interface{}
			_, err := store.Query(req.Context, func(e interface{}) (bool, error) {
				issues = append(issues, e.(map[string]interface{}))
				return true, nil
			})

			if err != nil {
				return err
			}

			for _, is := range issues {
				for i := range is {
					if i == "_id" {
						id := fmt.Sprint(is[i])
						_, err := store.Delete(req.Context, id)
						if err != nil {
							return err
						}
					}
				}
			}
		} else {
			_, err := store.Delete(req.Context, key)
			if err != nil {
				return err
			}
		}

		return nil
	},
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

	if dbAddress != dbAddressDemandSupply {
		_, err = db.Create(ctx, dbAddress, "keyvalue", &orbitdb.CreateDBOptions{})
		if err != nil {
			return db, nil, err
		}

		return db, nil, nil
	}

	store, err := db.KeyValue(ctx, dbAddress, &orbitdb.CreateDBOptions{})
	if err != nil {
		return db, nil, err
	}

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
					dbURI := store.Address().String()
					onReady(dbURI)
				}
			}
		}
	}()

	store.Load(ctx, -1)
	if err != nil {
		return db, nil, err
	}

	return db, store, nil
}

func ConnectDocs(ctx context.Context, dbAddress string, api iface.CoreAPI, onReady func(address string)) (orbitdb.OrbitDB, orbitdb.DocumentStore, error) {
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

	if dbAddress != dbAddressIssue && dbAddress != dbAddressCitizenReputation && dbAddress != dbAddressEvent {
		_, err = db.Create(ctx, dbAddress, "docstore", &orbitdb.CreateDBOptions{})
		if err != nil {
			return db, nil, err
		}

		return db, nil, nil
	}

	store, err := db.Docs(ctx, dbAddress, &orbitdb.CreateDBOptions{})
	if err != nil {
		return db, nil, err
	}

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
					dbURI := store.Address().String()
					onReady(dbURI)
				}
			}
		}
	}()

	store.Load(ctx, -1)
	if err != nil {
		return db, nil, err
	}

	return db, store, nil
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
