package postgresds

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	postgresdb "gx/ipfs/QmZmPKg1RKY2Cy5czKEBJTfDYzenbGww2pFNQKxLSKrPRB/sql-datastore/postgres"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&postgresdsPlugin{},
}

type postgresdsPlugin struct{}

var _ plugin.PluginDatastore = (*postgresdsPlugin)(nil)

func (*postgresdsPlugin) Name() string {
	return "ds-postgresds"
}

func (*postgresdsPlugin) Version() string {
	return "0.1.0"
}

func (*postgresdsPlugin) Init() error {
	return nil
}

func (*postgresdsPlugin) DatastoreTypeName() string {
	return "postgres"
}

type datastoreConfig struct {
	host     string
	port     string
	user     string
	passfile string
	password string
	dbname   string
}

// Returns a configuration stub for a postgres datastore from the given parameters
func (*postgresdsPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]interface{}) (fsrepo.DatastoreConfig, error) {
		var c datastoreConfig
		var ok bool
		c.passfile, ok = params["passfile"].(string)
		if !ok {
			return nil, fmt.Errorf("'passfile' field was not a string")
		}
		if c.passfile != "" {
			path, err := homedir.Expand(filepath.Clean(c.passfile))
			if err != nil {
				return nil, err
			}
			info, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, err
			}
			envVars := strings.Split(string(info), ":")
			if len(envVars) != 5 {
				return nil, fmt.Errorf("passfile at %s not of format: <IPFS_PGHOST>:<IPFS_PGPORT>:<IPFS_PGDATABASE>:<IPFS_PGUSER>:<IPFS_PGPASSWORD>", c.passfile)
			}
			c.host = envVars[0]
			c.port = envVars[1]
			c.dbname = envVars[2]
			c.user = envVars[3]
			c.password = envVars[4]
			return &c, nil
		}
		c.host, ok = params["host"].(string)
		if !ok {
			return nil, fmt.Errorf("'path' field was not a string")
		}
		c.port, ok = params["port"].(string)
		if !ok {
			return nil, fmt.Errorf("'port' field was not a string")
		}
		c.user, ok = params["user"].(string)
		if !ok {
			return nil, fmt.Errorf("'user' field was not a string")
		}
		c.dbname, ok = params["dbname"].(string)
		if !ok {
			return nil, fmt.Errorf("'dbname' field was not a string")
		}
		c.password, ok = params["password"].(string)
		if !ok {
			return nil, fmt.Errorf("'password' field was not a string")
		}
		return &c, nil
	}
}

func (c *datastoreConfig) DiskSpec() fsrepo.DiskSpec {
	return map[string]interface{}{
		"type":     "postgres",
		"user":     c.user,
		"database": c.dbname,
	}
}

func (c *datastoreConfig) Create(path string) (repo.Datastore, error) {
	pg := postgresdb.Options{
		Host:     c.host,
		User:     c.user,
		Database: c.dbname,
		Password: c.password,
		Port:     c.port,
	}
	ds, err := pg.Create()
	if err != nil {
		fmt.Println("error loading pg: ", err)
		return ds, err
	}
	return ds, nil
}
