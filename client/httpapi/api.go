package httpapi

import (
	"errors"
	"io/ioutil"
	gohttp "net/http"
	"os"
	"path"
	"strings"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	homedir "github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

const (
	DefaultPathName = ".ipfs"
	DefaultPathRoot = "~/" + DefaultPathName
	DefaultApiFile  = "api"
	EnvDir          = "IPFS_PATH"
)

var ErrNotImplemented = errors.New("not implemented")

type HttpApi struct {
	url     string
	httpcli *gohttp.Client
}

func NewLocalApi() iface.CoreAPI {
	baseDir := os.Getenv(EnvDir)
	if baseDir == "" {
		baseDir = DefaultPathRoot
	}

	baseDir, err := homedir.Expand(baseDir)
	if err != nil {
		return nil
	}

	apiFile := path.Join(baseDir, DefaultApiFile)

	if _, err := os.Stat(apiFile); err != nil {
		return nil
	}

	api, err := ioutil.ReadFile(apiFile)
	if err != nil {
		return nil
	}

	return NewApi(strings.TrimSpace(string(api)))
}

func NewApi(url string) *HttpApi {
	c := &gohttp.Client{
		Transport: &gohttp.Transport{
			Proxy:             gohttp.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}

	return NewApiWithClient(url, c)
}

func NewApiWithClient(url string, c *gohttp.Client) *HttpApi {
	if a, err := ma.NewMultiaddr(url); err == nil {
		_, host, err := manet.DialArgs(a)
		if err == nil {
			url = host
		}
	}

	return &HttpApi{
		url:     url,
		httpcli: c,
	}
}

func (api *HttpApi) request(command string, args ...string) *RequestBuilder {
	return &RequestBuilder{
		command: command,
		args:    args,
		shell:   api,
	}
}

func (api *HttpApi) Unixfs() iface.UnixfsAPI {
	return nil
}

func (api *HttpApi) Block() iface.BlockAPI {
	return (*BlockAPI)(api)
}

func (api *HttpApi) Dag() iface.DagAPI {
	return nil
}

func (api *HttpApi) Name() iface.NameAPI {
	return (*NameAPI)(api)
}

func (api *HttpApi) Key() iface.KeyAPI {
	return nil
}

func (api *HttpApi) Pin() iface.PinAPI {
	return nil
}

func (api *HttpApi) Object() iface.ObjectAPI {
	return nil
}

func (api *HttpApi) Dht() iface.DhtAPI {
	return nil
}

func (api *HttpApi) Swarm() iface.SwarmAPI {
	return nil
}

func (api *HttpApi) PubSub() iface.PubSubAPI {
	return nil
}
