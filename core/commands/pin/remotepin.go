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

	"golang.org/x/sync/errgroup"

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
const pinServiceStatOptionName = "stat"
const pinBackgroundOptionName = "background"
const pinForceOptionName = "force"

type RemotePinOutput struct {
	Status string
	Cid    string
	Name   string
}

func toRemotePinOutput(ps pinclient.PinStatusGetter) RemotePinOutput {
	return RemotePinOutput{
		Name:   ps.GetPin().GetName(),
		Status: ps.GetStatus().String(),
		Cid:    ps.GetPin().GetCid().String(),
	}
}

func printRemotePinDetails(w io.Writer, out *RemotePinOutput) {
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	defer tw.Flush()
	fw := func(k string, v string) {
		fmt.Fprintf(tw, "%s:\t%s\n", k, v)
	}
	fw("CID", out.Cid)
	fw("Name", out.Name)
	fw("Status", out.Status)
}

// remote pin commands

var pinServiceNameOption = cmds.StringOption(pinServiceNameOptionName, "Name of the remote pinning service to use.")

var addRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Pin object to remote pinning service.",
		ShortDescription: "Stores an IPFS object from a given path to a remote pinning service.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, false, "Path to object(s) to be pinned."),
	},
	Options: []cmds.Option{
		cmds.StringOption(pinNameOptionName, "An optional name for the pin."),
		pinServiceNameOption,
		cmds.BoolOption(pinBackgroundOptionName, "Add to the queue on the remote service and return immediately (does not wait for pinned status).").WithDefault(false),
	},
	Type: RemotePinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

		// Get remote service
		c, err := getRemotePinServiceFromRequest(req, env)
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
		if name, nameFound := req.Options[pinNameOptionName]; nameFound {
			nameStr := name.(string)
			opts = append(opts, pinclient.PinOpts.WithName(nameStr))
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
					return fmt.Errorf("waiting for pin interrupted, requestid=%q remains on remote service", requestId)
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

	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		cmds.StringOption(pinNameOptionName, "Return pins objects with names that contain provided value (case-sensitive, exact match)."),
		cmds.StringsOption(pinCIDsOptionName, "Return only pin objects for the specified CID(s); optional, comma separated."),
		cmds.StringsOption(pinStatusOptionName, "Return only pin objects with the specified statuses (queued,pinning,pinned,failed)").WithDefault([]string{"pinned"}),
		pinServiceNameOption,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

		c, err := getRemotePinServiceFromRequest(req, env)
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
			fmt.Fprintf(w, "%s\t%s\t%s\n", out.Cid, out.Status, cmdenv.EscNonPrint(out.Name))
			return nil
		}),
	},
}

// Executes GET /pins/?query-with-filters
func lsRemote(ctx context.Context, req *cmds.Request, c *pinclient.Client) (chan pinclient.PinStatusGetter, chan error, error) {
	opts := []pinclient.LsOption{}
	if name, nameFound := req.Options[pinNameOptionName]; nameFound {
		nameStr := name.(string)
		opts = append(opts, pinclient.PinOpts.FilterName(nameStr))
	}

	if cidsRaw, cidsFound := req.Options[pinCIDsOptionName]; cidsFound {
		cidsRawArr := cidsRaw.([]string)
		parsedCIDs := []cid.Cid{}
		for _, rawCID := range flattenCommaList(cidsRawArr) {
			parsedCID, err := cid.Decode(rawCID)
			if err != nil {
				return nil, nil, fmt.Errorf("CID %q cannot be parsed: %v", rawCID, err)
			}
			parsedCIDs = append(parsedCIDs, parsedCID)
		}
		opts = append(opts, pinclient.PinOpts.FilterCIDs(parsedCIDs...))
	}
	if statusRaw, statusFound := req.Options[pinStatusOptionName]; statusFound {
		statusRawArr := statusRaw.([]string)
		parsedStatuses := []pinclient.Status{}
		for _, rawStatus := range flattenCommaList(statusRawArr) {
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

	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		pinServiceNameOption,
		cmds.StringOption(pinNameOptionName, "Remove pin objects with names that contain provided value (case-sensitive, exact match)."),
		cmds.StringsOption(pinCIDsOptionName, "Remove only pin objects for the specified CID(s)."),
		cmds.StringsOption(pinStatusOptionName, "Remove only pin objects with the specified statuses (queued,pinning,pinned,failed).").WithDefault([]string{"pinned"}),
		cmds.BoolOption(pinForceOptionName, "Remove multiple pins without confirmation.").WithDefault(false),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

		c, err := getRemotePinServiceFromRequest(req, env)
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
			return fmt.Errorf("unexpected argument %q", req.Arguments[0])
		}

		for _, rmID := range rmIDs {
			if err := c.DeleteByID(ctx, rmID); err != nil {
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
	Type: nil,
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
		url := strings.TrimSuffix(req.Arguments[1], "/pins") // fix /pins/pins :-)
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
			Api: config.RemotePinningServiceApi{
				Endpoint: url,
				Key:      key,
			},
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
	Options: []cmds.Option{
		cmds.BoolOption(pinServiceStatOptionName, "Try to fetch and display current pin count on remote service (queued/pinning/pinned/failed).").WithDefault(false),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

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
			return cmds.EmitOnce(res, &PinServicesList{make([]ServiceDetails, 0)})
		}
		services := cfg.Pinning.RemoteServices
		result := PinServicesList{make([]ServiceDetails, 0, len(services))}
		for svcName, svcConfig := range services {
			svcDetails := ServiceDetails{svcName, svcConfig.Api.Endpoint, nil}

			// if --pin-count is passed, we try to fetch pin numbers from remote service
			if req.Options[pinServiceStatOptionName].(bool) {
				lsRemotePinCount := func(ctx context.Context, env cmds.Environment, svcName string) (*PinCount, error) {
					c, err := getRemotePinService(env, svcName)
					if err != nil {
						return nil, err
					}
					// we only care about total count, so requesting smallest batch
					batch := pinclient.PinOpts.Limit(1)
					fs := pinclient.PinOpts.FilterStatus

					statuses := []pinclient.Status{
						pinclient.StatusQueued,
						pinclient.StatusPinning,
						pinclient.StatusPinned,
						pinclient.StatusFailed,
					}

					g, ctx := errgroup.WithContext(ctx)
					pc := &PinCount{}

					for _, s := range statuses {
						status := s // lol https://golang.org/doc/faq#closures_and_goroutines
						g.Go(func() error {
							_, n, err := c.LsBatchSync(ctx, batch, fs(status))
							if err != nil {
								return err
							}
							switch status {
							case pinclient.StatusQueued:
								pc.Queued = n
							case pinclient.StatusPinning:
								pc.Pinning = n
							case pinclient.StatusPinned:
								pc.Pinned = n
							case pinclient.StatusFailed:
								pc.Failed = n
							}
							return nil
						})
					}
					if err := g.Wait(); err != nil {
						return nil, err
					}

					return pc, nil
				}

				pinCount, err := lsRemotePinCount(ctx, env, svcName)

				// PinCount is present only if we were able to fetch counts.
				// We don't want to break listing of services so this is best-effort.
				// (verbose err is returned by 'pin remote ls', if needed)
				svcDetails.Stat = &Stat{}
				if err == nil {
					svcDetails.Stat.Status = "valid"
					svcDetails.Stat.PinCount = pinCount
				} else {
					svcDetails.Stat.Status = "invalid"
				}
			}
			result.RemoteServices = append(result.RemoteServices, svcDetails)
		}
		sort.Sort(result)
		return cmds.EmitOnce(res, &result)
	},
	Type: PinServicesList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, list *PinServicesList) error {
			tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			withStat := req.Options[pinServiceStatOptionName].(bool)
			for _, s := range list.RemoteServices {
				if withStat {
					stat := s.Stat.Status
					pc := s.Stat.PinCount
					if s.Stat.PinCount != nil {
						stat = fmt.Sprintf("%d/%d/%d/%d", pc.Queued, pc.Pinning, pc.Pinned, pc.Failed)
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Service, s.ApiEndpoint, stat)
				} else {
					fmt.Fprintf(tw, "%s\t%s\n", s.Service, s.ApiEndpoint)
				}
			}
			tw.Flush()
			return nil
		}),
	},
}

type ServiceDetails struct {
	Service     string
	ApiEndpoint string
	Stat        *Stat `json:",omitempty"` // present only when --stat not passed
}

type Stat struct {
	Status   string
	PinCount *PinCount `json:",omitempty"` // missing when --stat is passed but the service is offline
}

type PinCount struct {
	Queued  int
	Pinning int
	Pinned  int
	Failed  int
}

// Struct returned by ipfs pin remote service ls --enc=json | jq
type PinServicesList struct {
	RemoteServices []ServiceDetails
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

func getRemotePinServiceFromRequest(req *cmds.Request, env cmds.Environment) (*pinclient.Client, error) {
	service, serviceFound := req.Options[pinServiceNameOptionName]
	if !serviceFound {
		return nil, fmt.Errorf("a service name must be passed")
	}

	serviceStr := service.(string)
	var err error
	c, err := getRemotePinService(env, serviceStr)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func getRemotePinService(env cmds.Environment, name string) (*pinclient.Client, error) {
	if name == "" {
		return nil, fmt.Errorf("remote pinning service name not specified")
	}
	url, key, err := getRemotePinServiceInfo(env, name)
	if err != nil {
		return nil, err
	}
	return pinclient.NewClient(url, key), nil
}

func getRemotePinServiceInfo(env cmds.Environment, name string) (url, key string, err error) {
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
	return service.Api.Endpoint, service.Api.Key, nil
}
