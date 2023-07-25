package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	iface "github.com/ipfs/boxo/coreiface"
	config "github.com/ipfs/kubo/config"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"

	cmds "github.com/stateless-minds/go-ipfs-cmds"
	orbitdb "github.com/stateless-minds/go-orbit-db"
	"github.com/stateless-minds/go-orbit-db/address"
	orbitdb_iface "github.com/stateless-minds/go-orbit-db/iface"
	"github.com/stateless-minds/go-orbit-db/stores"
)

const dbNameDemandSupply = "demand_supply"
const dbNameCitizenReputation = "citizen_reputation"
const dbNameIssue = "issue"
const dbNameEvent = "event"
const dbNameGift = "gift"

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
		cmds.StringArg("dbName", true, false, "DB address or name"),
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

		dbName := req.Arguments[0]
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

		db, store, err := ConnectKV(req.Context, dbName, api, func(address string) {})
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
		cmds.StringArg("dbName", true, false, "DB address or name"),
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

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		db, store, err := ConnectKV(req.Context, dbName, api, func(address string) {})
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
		cmds.StringArg("dbName", true, false, "DB address or name"),
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

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		db, store, err := ConnectKV(req.Context, dbName, api, func(address string) {})
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
		cmds.StringArg("dbName", true, false, "DB address or name"),
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

		dbName := req.Arguments[0]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
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
		cmds.StringArg("dbName", true, false, "DB address or name"),
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

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
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
		cmds.StringArg("dbName", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key to query"),
		cmds.StringArg("value", true, false, "Value to query"),
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

		dbName := req.Arguments[0]
		key := req.Arguments[1]
		value := req.Arguments[2]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		q, err := store.Query(req.Context, func(e interface{}) (bool, error) {
			record := e.(map[string]interface{})
			if key == "all" {
				return true, nil
			} else if strings.Contains(value, ",") {
				values := strings.Split(value, ",")
				recs, ok := record[key].(string)
				if !ok {
					return false, nil
				}
				if strings.Contains(recs, ",") {
					records := strings.Split(recs, ",")
					for _, r := range records {
						for _, v := range values {
							if r == v {
								return true, nil
							}
						}
					}
				}

			} else if record[key] == value {
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
		cmds.StringArg("dbName", true, false, "DB address or name"),
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

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			var records []map[string]interface{}
			_, err := store.Query(req.Context, func(e interface{}) (bool, error) {
				records = append(records, e.(map[string]interface{}))
				return true, nil
			})

			if err != nil {
				return err
			}

			for _, is := range records {
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

func ConnectKV(ctx context.Context, dbName string, api iface.CoreAPI, onReady func(address string)) (orbitdb.OrbitDB, orbitdb.KeyValueStore, error) {
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

	var addr address.Address
	switch dbName {
	case dbNameDemandSupply:
		addr, err = db.DetermineAddress(ctx, dbName, "keyvalue", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameDemandSupply, "keyvalue", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	default:
		// return if the dbName is not expected
		return db, nil, errors.New("unexpected dbName")
	}

	store, err := db.KeyValue(ctx, addr.String(), &orbitdb.CreateDBOptions{})
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

func ConnectDocs(ctx context.Context, dbName string, api iface.CoreAPI, onReady func(address string)) (orbitdb.OrbitDB, orbitdb.DocumentStore, error) {
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

	var addr address.Address
	switch dbName {
	case dbNameIssue:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameIssue, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbNameCitizenReputation:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameCitizenReputation, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbNameEvent:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameEvent, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbNameGift:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameGift, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	default:
		// return if the dbName is not expected
		return db, nil, errors.New("unexpected dbName")
	}

	store, err := db.Docs(ctx, addr.String(), &orbitdb.CreateDBOptions{})
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
