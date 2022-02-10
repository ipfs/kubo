package grpcds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	grpcds "github.com/guseggert/go-ds-grpc"
	pb "github.com/guseggert/go-ds-grpc/proto"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&grpcdsPlugin{},
}

type grpcdsPlugin struct{}

var _ plugin.PluginDatastore = (*grpcdsPlugin)(nil)

func (*grpcdsPlugin) Name() string {
	return "ds-grpc"
}

func (*grpcdsPlugin) Version() string {
	return "0.1.0"
}

func (*grpcdsPlugin) Init(_ *plugin.Environment) error {
	return nil
}

func (*grpcdsPlugin) DatastoreTypeName() string {
	return "grpcds"
}

type datastoreConfig struct {
	Name                 string          `json:"name"`
	Target               string          `json:"target"`
	AllowInsecure        bool            `json:"allowInsecure"`
	ConnectParams        *connectParams  `json:"connectParams"`
	Compressor           string          `json:"compressor"`
	DefaultServiceConfig json.RawMessage `json:"defaultServiceConfig"`
	UserAgent            string          `json:"userAgent"`
}

type connectParams struct {
	Backoff                 *backoffConfig `json:"backoff"`
	MinConnectTimeoutMillis int64          `json:"minConnectTimeoutMillis"`
}

type backoffConfig struct {
	BaseDelayMillis int64   `json:"baseDelayMillis"`
	Multiplier      float64 `json:"multiplier"`
	Jitter          float64 `json:"jitter"`
	MaxDelayMillis  int64   `json:"maxDelayMillis"`
}

func (b *backoffConfig) ToGRPCConfig() backoff.Config {
	return backoff.Config{
		BaseDelay:  time.Duration(b.BaseDelayMillis) * time.Millisecond,
		Multiplier: b.Multiplier,
		Jitter:     b.Jitter,
		MaxDelay:   time.Duration(b.MaxDelayMillis) * time.Millisecond,
	}
}

func (*grpcdsPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]interface{}) (fsrepo.DatastoreConfig, error) {

		var c datastoreConfig
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling grpcds config: %w", err)
		}

		err = json.Unmarshal(b, &c)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling grpcds config: %w", err)
		}

		if c.Name == "" {
			return nil, errors.New("'name' must be specified")
		}

		if c.Target == "" {
			return nil, errors.New("'target' must be specified")
		}

		return &c, nil
	}
}

func (c *datastoreConfig) DiskSpec() fsrepo.DiskSpec {
	return map[string]interface{}{
		"type": "grpcds",
		"name": c.Name,
	}
}

func (c *datastoreConfig) Create(path string) (repo.Datastore, error) {
	dialOpts := []grpc.DialOption{}
	callOpts := []grpc.CallOption{}

	if c.AllowInsecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if c.ConnectParams != nil {
		backoffConfig := backoff.DefaultConfig
		if c.ConnectParams.Backoff != nil {
			backoffConfig = c.ConnectParams.Backoff.ToGRPCConfig()
		}
		dialOpts = append(dialOpts, grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoffConfig,
			MinConnectTimeout: time.Duration(c.ConnectParams.MinConnectTimeoutMillis),
		}))
	}

	if c.DefaultServiceConfig != nil {
		dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(string(c.DefaultServiceConfig)))
	}

	if c.UserAgent != "" {
		dialOpts = append(dialOpts, grpc.WithUserAgent(c.UserAgent))
	}

	if c.Compressor != "" {
		callOpts = append(callOpts, grpc.UseCompressor(c.Compressor))
	}

	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(callOpts...))

	conn, err := grpc.Dial(c.Target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("initial dialing of grpcds target '%s': %w", c.Target, err)
	}
	client := pb.NewDatastoreClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ds, err := grpcds.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("building grpcds: %w", err)
	}
	repoDS, ok := ds.(repo.Datastore)
	if !ok {
		return nil, fmt.Errorf("remote gRPC datastore must be a repo datastore")
	}

	return repoDS, nil
}
