package pin

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	neturl "net/url"

	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	logging "github.com/ipfs/go-log"
	pinclient "github.com/ipfs/go-pinning-service-http-client"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("core/commands/cmdenv")

var remotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Pin (and unpin) objects to remote pinning service.",
	},

	Subcommands: map[string]*cmds.Command{
		"add":     addRemotePinCmd,
		"ls":      listRemotePinCmd,
		"rm":      rmRemotePinCmd,
		"service": remotePinServiceCmd,
	},
}

var remotePinServiceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Configure remote pinning services.",
	},

	Subcommands: map[string]*cmds.Command{
		"add": addRemotePinServiceCmd,
		"ls":  lsRemotePinServiceCmd,
		"rm":  rmRemotePinServiceCmd,
	},
}

const pinNameOptionName = "name"
const pinCIDsOptionName = "cid"
const pinStatusOptionName = "status"
const pinServiceNameOptionName = "service"
const pinServiceURLOptionName = "url"
const pinServiceKeyOptionName = "key"
const pinBackgroundOptionName = "background"
const pinForceOptionName = "force"

type RemotePinOutput struct {
	RequestID string
	Status    string
	Cid       string
	Name      string
	Delegates []string // multiaddr
}

func toRemotePinOutput(ps pinclient.PinStatusGetter) RemotePinOutput {
	return RemotePinOutput{
		RequestID: ps.GetRequestId(),
		Name:      ps.GetPin().GetName(),
		Delegates: multiaddrsToStrings(ps.GetDelegates()),
		Status:    ps.GetStatus().String(),
		Cid:       ps.GetPin().GetCid().String(),
	}
}

func printRemotePinDetails(w io.Writer, out *RemotePinOutput) {
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	defer tw.Flush()
	fw := func(k string, v string) {
		fmt.Fprintf(tw, "%s:\t%s\n", k, v)
	}
	fw("RequestID", out.RequestID)
	fw("CID", out.Cid)
	fw("Name", out.Name)
	fw("Status", out.Status)
}

// remote pin commands

var addRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Pin objects to remote storage.",
		ShortDescription: "Stores an IPFS object(s) from a given path to a remote pinning service.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, false, "Path to object(s) to be pinned."),
	},
	Options: []cmds.Option{
		cmds.StringOption(pinNameOptionName, "An optional name for the pin."),
		cmds.StringOption(pinServiceNameOptionName, "Name of the remote pinning service to use."),
		cmds.BoolOption(pinBackgroundOptionName, "Add to the queue on the remote service and return RequestID immediately.").WithDefault(false),
	},
	Type: RemotePinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

		// Get remote service
		service, _ := req.Options[pinServiceNameOptionName].(string)
		c, err := getRemotePinServiceOrEnv(env, service)
		if err != nil {
			return err
		}

		// Prepare value for Pin.cid
		if len(req.Arguments) != 1 {
			return fmt.Errorf("expecting one CID argument")
		}
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		rp, err := api.ResolvePath(ctx, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		// Prepare Pin.name
		opts := []pinclient.AddOption{}
		if name, nameFound := req.Options[pinNameOptionName].(string); nameFound {
			opts = append(opts, pinclient.PinOpts.WithName(name))
		}

		// Prepare Pin.origins
		// Add own multiaddrs to the 'origins' array, so Pinning Service can
		// use that as a hint and connect back to us (if possible)
		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}
		if node.PeerHost != nil {
			addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(node.PeerHost))
			if err != nil {
				return err
			}
			opts = append(opts, pinclient.PinOpts.WithOrigins(addrs...))
		}

		// Execute remote pin request
		// TODO: fix panic when pinning service is down
		ps, err := c.Add(ctx, rp.Cid(), opts...)
		if err != nil {
			return err
		}

		// Act on PinStatus.delegates
		// If Pinning Service returned any delegates, proactively try to
		// connect to them to facilitate data exchange without waiting for DHT
		// lookup
		for _, d := range ps.GetDelegates() {
			// TODO: confirm this works as expected
			p, err := peer.AddrInfoFromP2pAddr(d)
			if err != nil {
				return err
			}
			if err := api.Swarm().Connect(ctx, *p); err != nil {
				log.Infof("error connecting to remote pin delegate %v : %w", d, err)
			}
		}

		// Block unless --background=true is passed
		if !req.Options[pinBackgroundOptionName].(bool) {
			requestId := ps.GetRequestId()
			for {
				ps, err = c.GetStatusByID(ctx, requestId)
				if err != nil {
					return fmt.Errorf("failed to check pin status for requestid=%q due to error: %v", requestId, err)
				}
				if ps.GetRequestId() != requestId {
					return fmt.Errorf("failed to check pin status for requestid=%q, remote service sent unexpected requestid=%q", requestId, ps.GetRequestId())
				}
				s := ps.GetStatus()
				if s == pinclient.StatusPinned {
					break
				}
				if s == pinclient.StatusFailed {
					return fmt.Errorf("remote service failed to pin requestid=%q", requestId)
				}
				tmr := time.NewTimer(time.Second / 2)
				select {
				case <-tmr.C:
				case <-ctx.Done():
					return fmt.Errorf("waiting for pin interrupted, requestid=%q remains on remote service: check its status with 'ls' or abort with 'rm'", requestId)
				}
			}
		}

		return res.Emit(toRemotePinOutput(ps))
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *RemotePinOutput) error {
			printRemotePinDetails(w, out)
			return nil
		}),
	},
}

func multiaddrsToStrings(m []multiaddr.Multiaddr) []string {
	r := make([]string, len(m))
	for i := range m {
		r[i] = m[i].String()
	}
	return r
}

var listRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects pinned to remote pinning service.",
		ShortDescription: `
Returns a list of objects that are pinned to a remote pinning service.
`,
		LongDescription: `
Returns a list of objects that are pinned to a remote pinning service.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("request-id", false, true, "Request ID of the pin to be listed."),
	},
	Options: []cmds.Option{
		cmds.StringOption(pinNameOptionName, "Return pins objects with names that contain provided value (case-sensitive, exact match)."),
		cmds.StringsOption(pinCIDsOptionName, "Return only pin objects for the specified CID(s); optional, comma separated."),
		cmds.StringsOption(pinStatusOptionName, "Return only pin objects with the specified statuses (queued,pinning,pinned,failed)").WithDefault("pinned"),
		cmds.StringOption(pinServiceNameOptionName, "Name of the remote pinning service to use."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

		service, _ := req.Options[pinServiceNameOptionName].(string)
		c, err := getRemotePinServiceOrEnv(env, service)
		if err != nil {
			return err
		}

		psCh, errCh, err := lsRemote(ctx, req, c)
		if err != nil {
			return err
		}

		for ps := range psCh {
			if err := res.Emit(toRemotePinOutput(ps)); err != nil {
				return err
			}
		}

		return <-errCh
	},
	Type: RemotePinOutput{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *RemotePinOutput) error {
			// pin remote ls produces a flat output similar to legacy pin ls
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", out.RequestID, out.Cid, out.Status, out.Name)
			return nil
		}),
	},
}

// Returns pin objects with passed RequestIDs or matching filters.
func lsRemote(ctx context.Context, req *cmds.Request, c *pinclient.Client) (chan pinclient.PinStatusGetter, chan error, error) {
	// If request-ids are provided, do direct lookup for specific objects
	if len(req.Arguments) > 0 {
		return lsRemoteByRequestId(ctx, req, c)
	}
	// else, apply filters to find matching pin objects
	return lsRemoteWithFilters(ctx, req, c)
}

// Executes GET /pins/{requestid} for each requestid passed as argument.
// Status checks are executed one by one and operation is aborted on first error.
func lsRemoteByRequestId(ctx context.Context, req *cmds.Request, c *pinclient.Client) (chan pinclient.PinStatusGetter, chan error, error) {
	lsIDs := req.Arguments
	res := make(chan pinclient.PinStatusGetter, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(res)
		defer close(errs)
		for _, requestId := range lsIDs {
			ps, err := c.GetStatusByID(ctx, requestId)
			if err != nil {
				errs <- err
				return
			}
			select {
			case res <- ps:
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
		}
	}()
	return res, errs, nil
}

// Executes GET /pins/?query-with-filters
func lsRemoteWithFilters(ctx context.Context, req *cmds.Request, c *pinclient.Client) (chan pinclient.PinStatusGetter, chan error, error) {
	opts := []pinclient.LsOption{}
	if name, nameFound := req.Options[pinNameOptionName].(string); nameFound {
		opts = append(opts, pinclient.PinOpts.FilterName(name))
	}

	if cidsRaw, cidsFound := req.Options[pinCIDsOptionName].([]string); cidsFound {
		parsedCIDs := []cid.Cid{}
		for _, rawCID := range flattenCommaList(cidsRaw) {
			parsedCID, err := cid.Decode(rawCID)
			if err != nil {
				return nil, nil, fmt.Errorf("CID %q cannot be parsed: %v", rawCID, err)
			}
			parsedCIDs = append(parsedCIDs, parsedCID)
		}
		opts = append(opts, pinclient.PinOpts.FilterCIDs(parsedCIDs...))
	}
	if statusRaw, statusFound := req.Options[pinStatusOptionName].([]string); statusFound {
		parsedStatuses := []pinclient.Status{}
		for _, rawStatus := range flattenCommaList(statusRaw) {
			s := pinclient.Status(rawStatus)
			if s.String() == string(pinclient.StatusUnknown) {
				return nil, nil, fmt.Errorf("status %q is not valid", rawStatus)
			}
			parsedStatuses = append(parsedStatuses, s)
		}
		opts = append(opts, pinclient.PinOpts.FilterStatus(parsedStatuses...))
	}

	psCh, errCh := c.Ls(ctx, opts...)

	return psCh, errCh, nil
}

func flattenCommaList(list []string) []string {
	flatList := list[:0]
	for _, s := range list {
		flatList = append(flatList, strings.Split(s, ",")...)
	}
	return flatList
}

var rmRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove pinned objects from remote pinning service.",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("request-id", false, true, "Request ID of the pin to be removed."),
	},
	Options: []cmds.Option{
		cmds.StringOption(pinServiceNameOptionName, "Name of the remote pinning service to use."),
		cmds.StringOption(pinNameOptionName, "Remove pin objects with names that contain provided value (case-sensitive, exact match)."),
		cmds.StringsOption(pinCIDsOptionName, "Remove only pin objects for the specified CID(s)."),
		cmds.StringsOption(pinStatusOptionName, "Remove only pin objects with the specified statuses (queued,pinning,pinned,failed).").WithDefault("pinned"),
		cmds.BoolOption(pinForceOptionName, "Remove multiple pins without confirmation.").WithDefault(false),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

		service, _ := req.Options[pinServiceNameOptionName].(string)
		c, err := getRemotePinServiceOrEnv(env, service)
		if err != nil {
			return err
		}

		rmIDs := []string{}
		if len(req.Arguments) == 0 {
			psCh, errCh, err := lsRemote(ctx, req, c)
			if err != nil {
				return err
			}
			for ps := range psCh {
				rmIDs = append(rmIDs, ps.GetRequestId())
			}
			if err = <-errCh; err != nil {
				return fmt.Errorf("error while listing remote pins: %v", err)
			}

			if len(rmIDs) > 1 && !req.Options[pinForceOptionName].(bool) {
				return fmt.Errorf("multiple remote pins are matching this query, add --force to confirm the bulk removal")
			}
		} else {
			rmIDs = append(rmIDs, req.Arguments[0])
		}

		for _, rmID := range rmIDs {
			if err = c.DeleteByID(ctx, rmID); err != nil {
				return fmt.Errorf("removing pin identified by requestid=%q failed: %v", rmID, err)
			}
		}
		return nil
	},
}

// remote service commands

var addRemotePinServiceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Add remote pinning service.",
		ShortDescription: "Add a credentials for access to a remote pinning service.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg(pinServiceNameOptionName, true, false, "Service name."),
		cmds.StringArg(pinServiceURLOptionName, true, false, "Service URL."),
		cmds.StringArg(pinServiceKeyOptionName, true, false, "Service key."),
	},
	Options: []cmds.Option{},
	Type:    nil,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}
		repo, err := fsrepo.Open(cfgRoot)
		if err != nil {
			return err
		}
		defer repo.Close()

		if len(req.Arguments) < 3 {
			return fmt.Errorf("expecting three arguments: service name, url and key")
		}

		name := req.Arguments[0]
		url := req.Arguments[1]
		key := req.Arguments[2]

		u, err := neturl.ParseRequestURI(url)
		if err != nil || !strings.HasPrefix(u.Scheme, "http") {
			return fmt.Errorf("service url must be a valid HTTP URL")
		}

		cfg, err := repo.Config()
		if err != nil {
			return err
		}
		if cfg.Pinning.RemoteServices != nil {
			if _, present := cfg.Pinning.RemoteServices[name]; present {
				return fmt.Errorf("service already present")
			}
		} else {
			cfg.Pinning.RemoteServices = map[string]config.RemotePinningService{}
		}
		cfg.Pinning.RemoteServices[name] = config.RemotePinningService{
			ApiEndpoint: url,
			ApiKey:      key,
		}

		return repo.SetConfig(cfg)
	},
}

var rmRemotePinServiceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Remove remote pinning service.",
		ShortDescription: "Remove credentials for access to a remote pinning service.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("remote-pin-service", true, false, "Name of remote pinning service to remove."),
	},
	Options: []cmds.Option{},
	Type:    nil,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}
		repo, err := fsrepo.Open(cfgRoot)
		if err != nil {
			return err
		}
		defer repo.Close()

		if len(req.Arguments) != 1 {
			return fmt.Errorf("expecting one argument: name")
		}
		name := req.Arguments[0]

		cfg, err := repo.Config()
		if err != nil {
			return err
		}
		if cfg.Pinning.RemoteServices != nil {
			delete(cfg.Pinning.RemoteServices, name)
		}
		return repo.SetConfig(cfg)
	},
}

var lsRemotePinServiceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "List remote pinning services.",
		ShortDescription: "List remote pinning services.",
	},
	Arguments: []cmds.Argument{},
	Options:   []cmds.Option{
	// TODO: -s --stats that execute  ls query for each status  and reads pagination hints to return Stats object with the count of pins in each state: {Queued, Pinning, Pinned, Failed}
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}
		repo, err := fsrepo.Open(cfgRoot)
		if err != nil {
			return err
		}
		defer repo.Close()

		cfg, err := repo.Config()
		if err != nil {
			return err
		}
		if cfg.Pinning.RemoteServices == nil {
			return nil // no pinning services added yet
		}
		services := cfg.Pinning.RemoteServices
		result := PinServicesList{make([]PinServiceAndEndpoint, 0, len(services))}
		for svcName, svcConfig := range services {
			result.RemoteServices = append(result.RemoteServices, PinServiceAndEndpoint{svcName, svcConfig.ApiEndpoint})
		}
		sort.Sort(result)
		return cmds.EmitOnce(res, &result)
	},
	Type: PinServicesList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, list *PinServicesList) error {
			tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			for _, s := range list.RemoteServices {
				fmt.Fprintf(tw, "%s\t%s\n", s.Service, s.ApiEndpoint)
			}
			tw.Flush()
			return nil
		}),
	},
}

type PinServiceAndEndpoint struct {
	Service     string
	ApiEndpoint string
}

// Struct returned by ipfs pin remote service ls --enc=json | jq
type PinServicesList struct {
	RemoteServices []PinServiceAndEndpoint
}

func (l PinServicesList) Len() int {
	return len(l.RemoteServices)
}

func (l PinServicesList) Swap(i, j int) {
	s := l.RemoteServices
	s[i], s[j] = s[j], s[i]
}

func (l PinServicesList) Less(i, j int) bool {
	s := l.RemoteServices
	return s[i].Service < s[j].Service
}

func getRemotePinServiceOrEnv(env cmds.Environment, name string) (*pinclient.Client, error) {
	if name == "" {
		return nil, fmt.Errorf("remote pinning service name not specified")
	}
	url, key, err := getRemotePinService(env, name)
	if err != nil {
		return nil, err
	}
	return pinclient.NewClient(url, key), nil
}

func getRemotePinService(env cmds.Environment, name string) (url, key string, err error) {
	cfgRoot, err := cmdenv.GetConfigRoot(env)
	if err != nil {
		return "", "", err
	}
	repo, err := fsrepo.Open(cfgRoot)
	if err != nil {
		return "", "", err
	}
	defer repo.Close()
	cfg, err := repo.Config()
	if err != nil {
		return "", "", err
	}
	if cfg.Pinning.RemoteServices == nil {
		return "", "", fmt.Errorf("service not known")
	}
	service, present := cfg.Pinning.RemoteServices[name]
	if !present {
		return "", "", fmt.Errorf("service not known")
	}
	return service.ApiEndpoint, service.ApiKey, nil
}
