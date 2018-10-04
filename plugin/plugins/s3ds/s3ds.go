package s3ds

import (
	"fmt"
	"strconv"

	s3ds "gx/ipfs/QmYHDxm9sm7whjnbFMq8YdKVdQuji4858PGQRgXYjUZyNv/go-ds-s3"

	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

var Plugins = []plugin.Plugin{
	&s3Plugin{},
}

type s3Plugin struct{}

var _ plugin.PluginDatastore = (*s3Plugin)(nil)

func (s3p s3Plugin) Name() string {
	return "ds-s3"
}

func (s3p s3Plugin) Version() string {
	return "0.0.1"
}

func (s3p s3Plugin) Init() error {
	return nil
}

func (s3p s3Plugin) DatastoreTypeName() string {
	return "s3ds"
}

func (s3p s3Plugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(m map[string]interface{}) (fsrepo.DatastoreConfig, error) {
		accessKey := m["accessKey"]
		if accessKey == nil {
			accessKey = ""
		}

		secretKey := m["secretKey"]
		if secretKey == nil {
			secretKey = ""
		}

		sessionToken := m["sessionToken"]
		if sessionToken == nil {
			sessionToken = ""
		}

		bucket, ok := m["bucket"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no bucket specified")
		}

		region, ok := m["region"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no region specified")
		}

		regionEndpoint, ok := m["regionEndpoint"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no regionEndpoint specified")
		}

		rootDirectory, ok := m["rootDirectory"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no rootDir specified")
		}

		workersRaw := m["workers"]
		if workersRaw == nil {
			workersRaw = "0"
		}
		workers, err := strconv.Atoi(workersRaw.(string))
		if err != nil {
			return nil, fmt.Errorf("s3ds: workers is not an int '%s'", workersRaw)
		}

		return &S3Config{
			cfg: s3ds.Config{
				AccessKey:      accessKey.(string),
				SecretKey:      secretKey.(string),
				SessionToken:   sessionToken.(string),
				Bucket:         bucket,
				Region:         region,
				RegionEndpoint: regionEndpoint,
				RootDirectory:  rootDirectory,
				Workers:        workers,
			},
		}, nil
	}
}

type S3Config struct {
	cfg s3ds.Config
}

func (s3c *S3Config) DiskSpec() fsrepo.DiskSpec {
	return map[string]interface{}{
		"bucket":         s3c.cfg.Bucket,
		"region":         s3c.cfg.Region,
		"regionEndpoint": s3c.cfg.RegionEndpoint,
		"rootDirectory":  s3c.cfg.RootDirectory,
		"workers":        s3c.cfg.Workers,
	}
}

func (s3c *S3Config) Create(path string) (repo.Datastore, error) {
	return s3ds.NewS3Datastore(s3c.cfg)
}
