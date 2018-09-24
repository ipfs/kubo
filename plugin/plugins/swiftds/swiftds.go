package swiftds

import (
	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	"github.com/ipfs/go-ds-swift"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&swiftPlugin{},
}

type swiftPlugin struct{}

var _ plugin.PluginDatastore = (*swiftPlugin)(nil)

func (*swiftPlugin) Name() string {
	return "ds-swift"
}

func (*swiftPlugin) Version() string {
	return "0.0.1"
}

func (*swiftPlugin) Init() error {
	return nil
}

func (*swiftPlugin) DatastoreTypeName() string {
	return "swiftds"
}

func (*swiftPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(conf map[string]interface{}) (fsrepo.DatastoreConfig, error) {
		// v2 only for now

		c := &swiftConfig{}

		c.AuthUrl = conf["AuthUrl"].(string)
		c.TenantId = conf["TenantId"].(string)
		c.Tenant = conf["Tenant"].(string)
		c.UserName = conf["UserName"].(string)
		c.ApiKey = conf["ApiKey"].(string)
		c.Region = conf["Region"].(string)
		c.Container = conf["Container"].(string)

		return c, nil
	}
}

type swiftConfig struct {
	AuthUrl  string
	TenantId string
	Tenant   string

	UserName string
	ApiKey   string // password

	Region    string
	Container string
}

func (c *swiftConfig) DiskSpec() fsrepo.DiskSpec {
	// TODO: what else can we strip from here
	return map[string]interface{}{
		"AuthUrl":   c.AuthUrl,
		"TenantId":  c.TenantId,
		"Tenant":    c.Tenant,
		"UserName":  c.UserName,
		"ApiKey":    c.ApiKey,
		"Region":    c.Region,
		"Container": c.Container,
	}
}

func (c *swiftConfig) Create(path string) (repo.Datastore, error) {
	conf := swiftds.Config{}

	conf.AuthUrl = c.AuthUrl
	conf.TenantId = c.TenantId
	conf.Tenant = c.Tenant
	conf.UserName = c.UserName
	conf.ApiKey = c.ApiKey
	conf.Region = c.Region
	conf.Container = c.Container
	conf.AuthVersion = 2

	return swiftds.NewSwiftDatastore(conf)
}
