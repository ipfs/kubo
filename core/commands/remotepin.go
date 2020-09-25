package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	pinclient "github.com/ipfs/go-pinning-service-http-client"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

var remotePinURL = os.Getenv("IPFS_REMOTE_PIN_SERVICE")
var remotePinKey = os.Getenv("IPFS_REMOTE_PIN_KEY")

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
		// "rename": renameRemotePinServiceCmd,
		// "update": updateRemotePinServiceCmd,
		// "rm":     rmRemotePinServiceCmd,
	},
}

const pinNameOptionName = "name"
const pinCIDsOptionName = "cid"
const pinServiceNameOptionName = "service"

type AddRemotePinOutput struct {
	ID        string
	Name      string
	Delegates []string // multiaddr
	Status    string
	Cid       string
}

var addRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Pin objects to remote storage.",
		ShortDescription: "Stores an IPFS object(s) from a given path to a remote pinning service.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, false, "Path to object(s) to be pinned.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(pinNameOptionName, "An optional name for the pin."),
		cmds.StringsOption(pinServiceNameOptionName, "Name of the remote pinning service to use."),
	},
	Type: AddRemotePinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opts := []pinclient.AddOption{}
		if name, nameFound := req.Options[pinNameOptionName].(string); nameFound {
			opts = append(opts, pinclient.PinOpts.WithName(name))
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		if len(req.Arguments) != 1 {
			return fmt.Errorf("expecting one CID argument")
		}
		rp, err := api.ResolvePath(ctx, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		service, serviceFound := req.Options[pinServiceNameOptionName].(string)
		if !serviceFound {
			return fmt.Errorf("remote pinning service name not specified")
		}
		url, key, err := getRemotePinServiceOrEnv(env, service)
		if err != nil {
			return err
		}
		c := pinclient.NewClient(url, key)

		ps, err := c.Add(ctx, rp.Cid(), opts...)
		if err != nil {
			return err
		}

		for _, d := range ps.GetDelegates() {
			p, err := peer.AddrInfoFromP2pAddr(d)
			if err != nil {
				return err
			}
			if err := api.Swarm().Connect(ctx, *p); err != nil {
				log.Infof("error connecting to remote pin delegate %v : %w", d, err)
			}

		}

		return res.Emit(&AddRemotePinOutput{
			ID:        ps.GetRequestId(),
			Name:      ps.GetPin().GetName(),
			Delegates: multiaddrsToStrings(ps.GetDelegates()),
			Status:    ps.GetStatus().String(),
			Cid:       ps.GetPin().GetCid().String(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *AddRemotePinOutput) error {
			fmt.Printf("pin_id=%v\n", out.ID)
			fmt.Printf("pin_name=%q\n", out.Name)
			for _, d := range out.Delegates {
				fmt.Printf("pin_delegate=%v\n", d)
			}
			fmt.Printf("pin_status=%v\n", out.Status)
			fmt.Printf("pin_cid=%v\n", out.Cid)
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

	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		cmds.StringOption(pinNameOptionName, "Return pins objects with names that contain provided value (case-insensitive, partial or full match)."),
		cmds.StringsOption(pinCIDsOptionName, "Return only pin objects for the specified CID(s); optional, comma separated."),
		cmds.StringsOption(pinServiceNameOptionName, "Name of the remote pinning service to use."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opts := []pinclient.LsOption{}
		if name, nameFound := req.Options[pinNameOptionName].(string); nameFound {
			opts = append(opts, pinclient.PinOpts.FilterName(name))
		}
		if cidsRaw, cidsFound := req.Options[pinNameOptionName].([]string); cidsFound {
			parsedCIDs := []cid.Cid{}
			for _, rawCID := range cidsRaw {
				parsedCID, err := cid.Decode(rawCID)
				if err != nil {
					return fmt.Errorf("CID %s cannot be parsed (%v)", rawCID, err)
				}
				parsedCIDs = append(parsedCIDs, parsedCID)
			}
			opts = append(opts, pinclient.PinOpts.FilterCIDs(parsedCIDs...))
		}

		service, serviceFound := req.Options[pinServiceNameOptionName].(string)
		if !serviceFound {
			return fmt.Errorf("remote pinning service name not specified")
		}
		url, key, err := getRemotePinServiceOrEnv(env, service)
		if err != nil {
			return err
		}
		c := pinclient.NewClient(url, key)

		psCh, errCh := c.Ls(ctx, opts...)

		for ps := range psCh {
			if err := res.Emit(&AddRemotePinOutput{
				ID:        ps.GetRequestId(),
				Name:      ps.GetPin().GetName(),
				Delegates: multiaddrsToStrings(ps.GetDelegates()),
				Status:    ps.GetStatus().String(),
				Cid:       ps.GetPin().GetCid().String(),
			}); err != nil {
				return err
			}
		}

		return <-errCh
	},
	Type: AddRemotePinOutput{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *AddRemotePinOutput) error {
			fmt.Printf("pin_id=%v\n", out.ID)
			fmt.Printf("pin_name=%q\n", out.Name)
			for _, d := range out.Delegates {
				fmt.Printf("pin_delegate=%v\n", d)
			}
			fmt.Printf("pin_status=%v\n", out.Status)
			fmt.Printf("pin_cid=%v\n", out.Cid)
			return nil
		}),
	},
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
		cmds.StringArg("pin-id", true, true, "ID of the pin to be removed.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringsOption(pinServiceNameOptionName, "Name of the remote pinning service to use."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if len(req.Arguments) == 0 {
			return fmt.Errorf("missing a pin ID argument")
		}

		service, serviceFound := req.Options[pinServiceNameOptionName].(string)
		if !serviceFound {
			return fmt.Errorf("remote pinning service name not specified")
		}
		url, key, err := getRemotePinServiceOrEnv(env, service)
		if err != nil {
			return err
		}
		c := pinclient.NewClient(url, key)

		return c.DeleteByID(ctx, req.Arguments[0])
	},
}

var addRemotePinServiceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Add remote pinning service.",
		ShortDescription: "Add a credentials for access to a remote pinning service.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Name, URL and key (in that order) for a remote pinning service.").EnableStdin(),
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

		if len(req.Arguments) != 3 {
			return fmt.Errorf("expecting three argument: name, url and key")
		}
		name := req.Arguments[0]
		url := req.Arguments[1]
		key := req.Arguments[2]

		cfg, err := repo.Config()
		if err != nil {
			return err
		}
		if cfg.RemotePinServices.Services != nil {
			if _, present := cfg.RemotePinServices.Services[name]; present {
				return fmt.Errorf("service already present")
			}
		} else {
			cfg.RemotePinServices.Services = map[string]config.RemotePinService{}
		}
		cfg.RemotePinServices.Services[name] = config.RemotePinService{
			Name: name,
			URL:  url,
			Key:  key,
		}

		return repo.SetConfig(cfg)
	},
}

func getRemotePinServiceOrEnv(env cmds.Environment, name string) (url, key string, err error) {
	if remotePinURL != "" && remotePinKey != "" {
		return remotePinURL, remotePinKey, nil
	}
	return getRemotePinService(env, name)
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
	if cfg.RemotePinServices.Services == nil {
		return "", "", fmt.Errorf("service not known")
	}
	service, present := cfg.RemotePinServices.Services[name]
	if !present {
		return "", "", fmt.Errorf("service not known")
	}
	return service.URL, service.Key, nil
}
