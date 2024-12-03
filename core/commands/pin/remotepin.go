package pin

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	neturl "net/url"
	gopath "path"

	"golang.org/x/sync/errgroup"

	pinclient "github.com/ipfs/boxo/pinning/remote/client"
	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
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

const (
	pinNameOptionName         = "name"
	pinCIDsOptionName         = "cid"
	pinStatusOptionName       = "status"
	pinServiceNameOptionName  = "service"
	pinServiceNameArgName     = pinServiceNameOptionName
	pinServiceEndpointArgName = "endpoint"
	pinServiceKeyArgName      = "key"
	pinServiceStatOptionName  = "stat"
	pinBackgroundOptionName   = "background"
	pinForceOptionName        = "force"
)

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

var pinServiceNameOption = cmds.StringOption(pinServiceNameOptionName, "Name of the remote pinning service to use (mandatory).")

var addRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Pin object to remote pinning service.",
		ShortDescription: "Asks remote pinning service to pin an IPFS object from a given path.",
		LongDescription: `
Asks remote pinning service to pin an IPFS object from a given path or a CID.

To pin CID 'bafkqaaa' to service named 'mysrv' under a pin named 'mypin':

  $ ipfs pin remote add --service=mysrv --name=mypin bafkqaaa

The above command will block until remote service returns 'pinned' status,
which may take time depending on the size and available providers of the pinned
data.

If you prefer to not wait for pinning confirmation and return immediately
after remote service confirms 'queued' status, add the '--background' flag:

  $ ipfs pin remote add --service=mysrv --name=mypin --background bafkqaaa

Status of background pin requests can be inspected with the 'ls' command.

To list all pins for the CID across all statuses:

  $ ipfs pin remote ls --service=mysrv --cid=bafkqaaa --status=queued \
      --status=pinning --status=pinned --status=failed

NOTE: a comma-separated notation is supported in CLI for convenience:

  $ ipfs pin remote ls --service=mysrv --cid=bafkqaaa --status=queued,pinning,pinned,failed

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, false, "CID or Path to be pinned."),
	},
	Options: []cmds.Option{
		pinServiceNameOption,
		cmds.StringOption(pinNameOptionName, "An optional name for the pin."),
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
		p, err := cmdutils.PathOrCidPath(req.Arguments[0])
		if err != nil {
			return err
		}

		rp, _, err := api.ResolvePath(ctx, p)
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
		// If CID in blockstore, add own multiaddrs to the 'origins' array
		// so pinning service can use that as a hint and connect back to us.
		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		isInBlockstore, err := node.Blockstore.Has(req.Context, rp.RootCid())
		if err != nil {
			return err
		}

		if isInBlockstore && node.PeerHost != nil {
			addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(node.PeerHost))
			if err != nil {
				return err
			}
			opts = append(opts, pinclient.PinOpts.WithOrigins(addrs...))
		} else if isInBlockstore && !node.IsOnline && cmds.GetEncoding(req, cmds.Text) == cmds.Text {
			fmt.Fprintf(os.Stdout, "WARNING: the local node is offline and remote pinning may fail if there is no other provider for this CID\n")
		}

		// Execute remote pin request
		// TODO: fix panic when pinning service is down
		ps, err := c.Add(ctx, rp.RootCid(), opts...)
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
			const pinWaitTime = 500 * time.Millisecond
			var timer *time.Timer
			requestID := ps.GetRequestId()
			for {
				ps, err = c.GetStatusByID(ctx, requestID)
				if err != nil {
					return fmt.Errorf("failed to check pin status for requestid=%q due to error: %v", requestID, err)
				}
				if ps.GetRequestId() != requestID {
					return fmt.Errorf("failed to check pin status for requestid=%q, remote service sent unexpected requestid=%q", requestID, ps.GetRequestId())
				}
				s := ps.GetStatus()
				if s == pinclient.StatusPinned {
					break
				}
				if s == pinclient.StatusFailed {
					return fmt.Errorf("remote service failed to pin requestid=%q", requestID)
				}
				if timer == nil {
					timer = time.NewTimer(pinWaitTime)
				} else {
					timer.Reset(pinWaitTime)
				}
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return fmt.Errorf("waiting for pin interrupted, requestid=%q remains on remote service", requestID)
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

NOTE: By default, it will only show matching objects in 'pinned' state.
Pass '--status=queued,pinning,pinned,failed' to list pins in all states.
`,
	},

	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		pinServiceNameOption,
		cmds.StringOption(pinNameOptionName, "Return pins with names that contain the value provided (case-sensitive, exact match)."),
		cmds.DelimitedStringsOption(",", pinCIDsOptionName, "Return pins for the specified CIDs (comma-separated)."),
		cmds.DelimitedStringsOption(",", pinStatusOptionName, "Return pins with the specified statuses (queued,pinning,pinned,failed).").WithDefault([]string{"pinned"}),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		c, err := getRemotePinServiceFromRequest(req, env)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

		psCh := make(chan pinclient.PinStatusGetter)
		lsErr := make(chan error, 1)
		go func() {
			lsErr <- lsRemote(ctx, req, c, psCh)
		}()
		for ps := range psCh {
			if err := res.Emit(toRemotePinOutput(ps)); err != nil {
				return err
			}
		}

		return <-lsErr
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
func lsRemote(ctx context.Context, req *cmds.Request, c *pinclient.Client, out chan<- pinclient.PinStatusGetter) error {
	opts := []pinclient.LsOption{}
	if name, nameFound := req.Options[pinNameOptionName]; nameFound {
		nameStr := name.(string)
		opts = append(opts, pinclient.PinOpts.FilterName(nameStr))
	}

	if cidsRaw, cidsFound := req.Options[pinCIDsOptionName]; cidsFound {
		cidsRawArr := cidsRaw.([]string)
		var parsedCIDs []cid.Cid
		for _, rawCID := range cidsRawArr {
			parsedCID, err := cid.Decode(rawCID)
			if err != nil {
				close(out)
				return fmt.Errorf("CID %q cannot be parsed: %v", rawCID, err)
			}
			parsedCIDs = append(parsedCIDs, parsedCID)
		}
		opts = append(opts, pinclient.PinOpts.FilterCIDs(parsedCIDs...))
	}
	if statusRaw, statusFound := req.Options[pinStatusOptionName]; statusFound {
		statusRawArr := statusRaw.([]string)
		var parsedStatuses []pinclient.Status
		for _, rawStatus := range statusRawArr {
			s := pinclient.Status(rawStatus)
			if s.String() == string(pinclient.StatusUnknown) {
				close(out)
				return fmt.Errorf("status %q is not valid", rawStatus)
			}
			parsedStatuses = append(parsedStatuses, s)
		}
		opts = append(opts, pinclient.PinOpts.FilterStatus(parsedStatuses...))
	}

	return c.Ls(ctx, out, opts...)
}

var rmRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Remove pins from remote pinning service.",
		ShortDescription: "Removes the remote pin, allowing it to be garbage-collected if needed.",
		LongDescription: `
Removes remote pins, allowing them to be garbage-collected if needed.

This command accepts the same search query parameters as 'ls', and it is good
practice to execute 'ls' before 'rm' to confirm the list of pins to be removed.

To remove a single pin for a specific CID:

  $ ipfs pin remote ls --service=mysrv --cid=bafkqaaa
  $ ipfs pin remote rm --service=mysrv --cid=bafkqaaa

When more than one pin matches the query on the remote service, an error is
returned.  To confirm the removal of multiple pins, pass '--force':

  $ ipfs pin remote ls --service=mysrv --name=popular-name
  $ ipfs pin remote rm --service=mysrv --name=popular-name --force

NOTE: When no '--status' is passed, implicit '--status=pinned' is used.
To list and then remove all pending pin requests, pass an explicit status list:

  $ ipfs pin remote ls --service=mysrv --status=queued,pinning,failed
  $ ipfs pin remote rm --service=mysrv --status=queued,pinning,failed --force

`,
	},

	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		pinServiceNameOption,
		cmds.StringOption(pinNameOptionName, "Remove pins with names that contain provided value (case-sensitive, exact match)."),
		cmds.DelimitedStringsOption(",", pinCIDsOptionName, "Remove pins for the specified CIDs."),
		cmds.DelimitedStringsOption(",", pinStatusOptionName, "Remove pins with the specified statuses (queued,pinning,pinned,failed).").WithDefault([]string{"pinned"}),
		cmds.BoolOption(pinForceOptionName, "Allow removal of multiple pins matching the query without additional confirmation.").WithDefault(false),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		c, err := getRemotePinServiceFromRequest(req, env)
		if err != nil {
			return err
		}

		rmIDs := []string{}
		if len(req.Arguments) != 0 {
			return fmt.Errorf("unexpected argument %q", req.Arguments[0])
		}

		psCh := make(chan pinclient.PinStatusGetter)
		errCh := make(chan error, 1)
		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()

		go func() {
			errCh <- lsRemote(ctx, req, c, psCh)
		}()
		for ps := range psCh {
			rmIDs = append(rmIDs, ps.GetRequestId())
		}
		if err = <-errCh; err != nil {
			return fmt.Errorf("error while listing remote pins: %v", err)
		}

		if len(rmIDs) > 1 && !req.Options[pinForceOptionName].(bool) {
			return fmt.Errorf("multiple remote pins are matching this query, add --force to confirm the bulk removal")
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
		ShortDescription: "Add credentials for access to a remote pinning service.",
		LongDescription: `
Add credentials for access to a remote pinning service and store them in the
config under Pinning.RemoteServices map.

TIP:

  To add services and test them by fetching pin count stats:

  $ ipfs pin remote service add goodsrv https://pin-api.example.com secret-key
  $ ipfs pin remote service add badsrv  https://bad-api.example.com invalid-key
  $ ipfs pin remote service ls --stat
  goodsrv   https://pin-api.example.com 0/0/0/0
  badsrv    https://bad-api.example.com invalid

`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg(pinServiceNameArgName, true, false, "Service name."),
		cmds.StringArg(pinServiceEndpointArgName, true, false, "Service endpoint."),
		cmds.StringArg(pinServiceKeyArgName, true, false, "Service key."),
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
			return fmt.Errorf("expecting three arguments: service name, endpoint and key")
		}

		name := req.Arguments[0]
		endpoint, err := normalizeEndpoint(req.Arguments[1])
		if err != nil {
			return err
		}
		key := req.Arguments[2]

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
			API: config.RemotePinningServiceAPI{
				Endpoint: endpoint,
				Key:      key,
			},
			Policies: config.RemotePinningServicePolicies{},
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
		cmds.StringArg(pinServiceNameOptionName, true, false, "Name of remote pinning service to remove."),
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
		LongDescription: `
List remote pinning services.

By default, only a name and an endpoint are listed; however, one can pass
'--stat' to test each endpoint by fetching pin counts for each state:

  $ ipfs pin remote service ls --stat
  goodsrv   https://pin-api.example.com 0/0/0/0
  badsrv    https://bad-api.example.com invalid

TIP: pass '--enc=json' for more useful JSON output.
`,
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
			svcDetails := ServiceDetails{svcName, svcConfig.API.Endpoint, nil}

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
	ApiEndpoint string //nolint
	Stat        *Stat  `json:",omitempty"` // present only when --stat not passed
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
	endpoint, key, err := getRemotePinServiceInfo(env, name)
	if err != nil {
		return nil, err
	}
	return pinclient.NewClient(endpoint, key), nil
}

func getRemotePinServiceInfo(env cmds.Environment, name string) (endpoint, key string, err error) {
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
	endpoint, err = normalizeEndpoint(service.API.Endpoint)
	if err != nil {
		return "", "", err
	}
	return endpoint, service.API.Key, nil
}

func normalizeEndpoint(endpoint string) (string, error) {
	uri, err := neturl.ParseRequestURI(endpoint)
	if err != nil || !(uri.Scheme == "http" || uri.Scheme == "https") {
		return "", fmt.Errorf("service endpoint must be a valid HTTP URL")
	}

	// cleanup trailing and duplicate slashes (https://github.com/ipfs/kubo/issues/7826)
	uri.Path = gopath.Clean(uri.Path)
	uri.Path = strings.TrimSuffix(uri.Path, ".")
	uri.Path = strings.TrimSuffix(uri.Path, "/")

	// remove any query params
	if uri.RawQuery != "" {
		return "", fmt.Errorf("service endpoint should be provided without any query parameters")
	}

	if strings.HasSuffix(uri.Path, "/pins") {
		return "", fmt.Errorf("service endpoint should be provided without the /pins suffix")
	}

	return uri.String(), nil
}
