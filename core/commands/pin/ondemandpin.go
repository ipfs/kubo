package pin

import (
	"context"
	"fmt"
	"io"
	"time"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/config"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	"github.com/ipfs/kubo/ondemandpin"
	"golang.org/x/sync/errgroup"
)

const (
	onDemandLiveOptionName = "live"
	liveLookupParallelism  = 8
)

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

			// Check the store first so an unregistered CID errors before any pin is touched.
			if _, err := store.Get(req.Context, c); err != nil {
				return err
			}

			// Only remove pins carrying the Kubo-internal kubo:on-demand name; user pins on the same CID are kept.
			hasOnDemandPin, err := ondemandpin.PinHasName(req.Context, n.Pinning, c, ondemandpin.OnDemandPinName)
			if err != nil {
				return fmt.Errorf("checking pin state for %s: %w", c, err)
			}
			if hasOnDemandPin {
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
	Cid               string `json:"Cid"`
	HasOnDemandPin    bool   `json:"HasOnDemandPin"`
	Providers         *int   `json:"Providers,omitempty"` // live lookup only
	ProvidersUnknown  bool   `json:"ProvidersUnknown,omitempty"`
	LastProviderCount *int   `json:"LastProviderCount,omitempty"`
	LastCheckedAt     string `json:"LastCheckedAt,omitempty"`
	LastResult        string `json:"LastResult,omitempty"`
	LastAboveTarget   string `json:"LastAboveTarget,omitempty"`
	UnpinAt           string `json:"UnpinAt,omitempty"`
	FailureCount      int    `json:"FailureCount,omitempty"`
	NextCheckAt       string `json:"NextCheckAt,omitempty"`
	CreatedAt         string `json:"CreatedAt"`
}

var listOnDemandPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List on-demand pins.",
		ShortDescription: `
Lists registered CIDs with last check result, provider count, and unpin time.
Use --live for a fresh DHT provider count (requires content routing).
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

		var replicationMin, replicationMax int
		if live {
			if n.Routing == nil {
				return fmt.Errorf("--live requires content routing; none is available")
			}
			cfg, err := n.Repo.Config()
			if err != nil {
				return err
			}
			replicationMin = int(cfg.OnDemandPinning.ReplicationTargetMin.WithDefault(config.DefaultOnDemandPinReplicationTargetMin))
			replicationMax = int(cfg.OnDemandPinning.ReplicationTargetMax.WithDefault(config.DefaultOnDemandPinReplicationTargetMax))
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

		type liveResult struct {
			count int
			ok    bool
		}
		liveResults := make([]liveResult, len(records))
		if live && len(records) > 0 {
			g, gctx := errgroup.WithContext(req.Context)
			g.SetLimit(liveLookupParallelism)
			for i := range records {
				i := i
				c := records[i].Cid
				g.Go(func() error {
					lookupCtx, cancel := context.WithTimeout(gctx, ondemandpin.CheckTimeout)
					defer cancel()
					count, ok := ondemandpin.CountProviders(lookupCtx, n.Routing, n.Identity, c, replicationMin, replicationMax)
					liveResults[i] = liveResult{count: count, ok: ok}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
		}

		for i, rec := range records {
			hasOnDemandPin, err := ondemandpin.PinHasName(req.Context, n.Pinning, rec.Cid, ondemandpin.OnDemandPinName)
			if err != nil {
				return fmt.Errorf("checking pin state for %s: %w", rec.Cid, err)
			}
			out := OnDemandLsOutput{
				Cid:            rec.Cid.String(),
				HasOnDemandPin: hasOnDemandPin,
				LastResult:     rec.LastResult,
				FailureCount:   rec.FailureCount,
				CreatedAt:      rec.CreatedAt.Format(time.RFC3339),
			}
			if !rec.LastCheckedAt.IsZero() {
				out.LastCheckedAt = rec.LastCheckedAt.Format(time.RFC3339)
			}
			if rec.LastResult != "" || !rec.LastCheckedAt.IsZero() {
				count := rec.LastProviderCount
				out.LastProviderCount = &count
			}
			if !rec.LastAboveTarget.IsZero() {
				out.LastAboveTarget = rec.LastAboveTarget.Format(time.RFC3339)
			}
			if !rec.UnpinAt.IsZero() {
				out.UnpinAt = rec.UnpinAt.Format(time.RFC3339)
			}
			if !rec.NextCheckAt.IsZero() {
				out.NextCheckAt = rec.NextCheckAt.Format(time.RFC3339)
			}
			if live {
				if liveResults[i].ok {
					count := liveResults[i].count
					out.Providers = &count
				} else {
					out.ProvidersUnknown = true
				}
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
			if out.HasOnDemandPin {
				pinState = "pinned"
			}
			fmt.Fprintf(w, "%s  %s", out.Cid, pinState)
			if out.Providers != nil {
				fmt.Fprintf(w, "  providers=%d", *out.Providers)
			} else if out.ProvidersUnknown {
				fmt.Fprintf(w, "  providers=unknown")
			} else if out.LastProviderCount != nil {
				fmt.Fprintf(w, "  last-providers=%d", *out.LastProviderCount)
			}
			if out.LastResult != "" {
				fmt.Fprintf(w, "  result=%s", out.LastResult)
			}
			if out.LastCheckedAt != "" {
				fmt.Fprintf(w, "  checked=%s", out.LastCheckedAt)
			}
			if out.UnpinAt != "" {
				fmt.Fprintf(w, "  unpin-at=%s", out.UnpinAt)
			}
			if out.NextCheckAt != "" {
				fmt.Fprintf(w, "  next-check=%s", out.NextCheckAt)
			}
			if out.FailureCount > 0 {
				fmt.Fprintf(w, "  failures=%d", out.FailureCount)
			}
			fmt.Fprintf(w, "  created=%s", out.CreatedAt)
			fmt.Fprintln(w)
			return nil
		}),
	},
}
