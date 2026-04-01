package pin

import (
	"fmt"
	"io"
	"time"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/config"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	"github.com/ipfs/kubo/ondemandpin"
)

const onDemandLiveOptionName = "live"

var onDemandPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage on-demand pins.",
		ShortDescription: `
On-demand pins when few DHT providers exist in the routing table; unpins after
replication stays above target for a grace period. Requires config
Experimental.OnDemandPinningEnabled.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"add": addOnDemandPinCmd,
		"rm":  rmOnDemandPinCmd,
		"ls":  listOnDemandPinCmd,
	},
}

type OnDemandPinOutput struct {
	Cid string `json:"Cid"`
}

var addOnDemandPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Register CIDs for on-demand pinning.",
		ShortDescription: `Registers CID(s) for on-demand pinning; checker pins when needed.`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "CID(s) to register."),
	},
	Type: OnDemandPinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		cfg, err := n.Repo.Config()
		if err != nil {
			return err
		}
		if !cfg.Experimental.OnDemandPinningEnabled {
			return fmt.Errorf("on-demand pinning is not enabled; set Experimental.OnDemandPinningEnabled = true in config")
		}

		store := n.OnDemandPinStore

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		for _, arg := range req.Arguments {
			p, err := cmdutils.PathOrCidPath(arg)
			if err != nil {
				return fmt.Errorf("invalid CID or path %q: %w", arg, err)
			}

			rp, _, err := api.ResolvePath(req.Context, p)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", arg, err)
			}
			c := rp.RootCid()

			if err := store.Add(req.Context, c); err != nil {
				return err
			}

			if checker := n.OnDemandPinChecker; checker != nil {
				checker.Enqueue(c)
			}

			if err := res.Emit(&OnDemandPinOutput{
				Cid: c.String(),
			}); err != nil {
				return err
			}
		}
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *OnDemandPinOutput) error {
			fmt.Fprintf(w, "registered %s for on-demand pinning\n", out.Cid)
			return nil
		}),
	},
}

var rmOnDemandPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove CIDs from on-demand pinning.",
		ShortDescription: `
Removes CID(s) from the registry. Checker-pinned content is unpinned.

Works when on-demand pinning is disabled, to clear old registrations.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "CID(s) to remove."),
	},
	Type: OnDemandPinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		store := n.OnDemandPinStore

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		for _, arg := range req.Arguments {
			p, err := cmdutils.PathOrCidPath(arg)
			if err != nil {
				return fmt.Errorf("invalid CID or path %q: %w", arg, err)
			}

			rp, _, err := api.ResolvePath(req.Context, p)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", arg, err)
			}
			c := rp.RootCid()

			isOurs, err := ondemandpin.PinHasName(req.Context, n.Pinning, c, ondemandpin.OnDemandPinName)
			if err != nil {
				return fmt.Errorf("checking pin state for %s: %w", c, err)
			}
			if isOurs {
				if err := api.Pin().Rm(req.Context, rp); err != nil {
					return fmt.Errorf("unpinning %s: %w", c, err)
				}
			}

			if err := store.Remove(req.Context, c); err != nil {
				return err
			}

			if err := res.Emit(&OnDemandPinOutput{
				Cid: c.String(),
			}); err != nil {
				return err
			}
		}
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *OnDemandPinOutput) error {
			fmt.Fprintf(w, "removed %s from on-demand pinning\n", out.Cid)
			return nil
		}),
	},
}

type OnDemandLsOutput struct {
	Cid             string `json:"Cid"`
	PinnedByUs      bool   `json:"PinnedByUs"`
	Providers       *int   `json:"Providers,omitempty"`
	LastAboveTarget string `json:"LastAboveTarget,omitempty"`
	CreatedAt       string `json:"CreatedAt"`
}

var listOnDemandPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List on-demand pins.",
		ShortDescription: `
Lists CIDs registered for on-demand pinning with their current state.
Use --live to include real-time provider counts from the DHT.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", false, true, "Optional CID(s) to filter."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(onDemandLiveOptionName, "l", "Perform live provider lookup."),
	},
	Type: OnDemandLsOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		store := n.OnDemandPinStore

		live, _ := req.Options[onDemandLiveOptionName].(bool)

		var globalTarget int
		if live {
			cfg, err := n.Repo.Config()
			if err != nil {
				return err
			}
			globalTarget = int(cfg.OnDemandPinning.ReplicationTarget.WithDefault(config.DefaultOnDemandPinReplicationTarget))
		}

		var records []ondemandpin.Record
		if len(req.Arguments) > 0 {
			api, err := cmdenv.GetApi(env, req)
			if err != nil {
				return err
			}
			for _, arg := range req.Arguments {
				p, err := cmdutils.PathOrCidPath(arg)
				if err != nil {
					return fmt.Errorf("invalid CID or path %q: %w", arg, err)
				}
				rp, _, err := api.ResolvePath(req.Context, p)
				if err != nil {
					return fmt.Errorf("resolving %q: %w", arg, err)
				}
				rec, err := store.Get(req.Context, rp.RootCid())
				if err != nil {
					return err
				}
				records = append(records, *rec)
			}
		} else {
			records, err = store.List(req.Context)
			if err != nil {
				return err
			}
		}

		for _, rec := range records {
			out := OnDemandLsOutput{
				Cid:        rec.Cid.String(),
				PinnedByUs: rec.PinnedByUs,
				CreatedAt:  rec.CreatedAt.Format(time.RFC3339),
			}
			if !rec.LastAboveTarget.IsZero() {
				out.LastAboveTarget = rec.LastAboveTarget.Format(time.RFC3339)
			}

			if live && n.Routing != nil {
				count := ondemandpin.CountProviders(req.Context, n.Routing, n.Identity, rec.Cid, globalTarget)
				out.Providers = &count
			}

			if err := res.Emit(&out); err != nil {
				return err
			}
		}
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *OnDemandLsOutput) error {
			pinState := "not-pinned"
			if out.PinnedByUs {
				pinState = "pinned"
			}
			fmt.Fprintf(w, "%s", out.Cid)
			if out.Providers != nil {
				fmt.Fprintf(w, "  providers=%d", *out.Providers)
			}
			fmt.Fprintf(w, "  %s  created=%s", pinState, out.CreatedAt)
			if out.LastAboveTarget != "" {
				fmt.Fprintf(w, "  above-target-since=%s", out.LastAboveTarget)
			}
			fmt.Fprintln(w)
			return nil
		}),
	},
}
