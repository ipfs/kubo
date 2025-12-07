package name

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ipfs/boxo/ipns"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
)

const (
	followIntervalOptionName = "interval"
	followOnceOptionName     = "once"
	followVerboseOptionName  = "verbose"
)

// FollowResult represents the output of the follow command.
type FollowResult struct {
	Key       string
	Status    string
	Timestamp string
	Error     string `json:",omitempty"`
}

var FollowCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Keep an IPNS record alive by periodically republishing it to the DHT.",
		ShortDescription: `
Periodically republishes an existing signed IPNS record to the DHT without 
needing the private key. This keeps the record available in the network
even if the original publisher is offline.
`,
		LongDescription: `
This command fetches an IPNS record from the DHT and periodically republishes 
it to keep it alive. The record is treated as opaque signed data - no 
modifications are made, and no private key is required.

Note: The IPNS record must already exist in the DHT and must not be expired.
You cannot extend the validity period of the record - you can only keep
republishing the existing signed record.

Examples:

Follow an IPNS name with default 30-minute interval:

  > ipfs name follow k51qzi5uqu5djwbl5v5r19lpxai8gxg0mtpgomn7s1ycfcpht8gmu85w3xyxzf

Follow with a custom interval:

  > ipfs name follow --interval=15m k51qzi5uqu5djwbl5v5r19lpxai8gxg0mtpgomn7s1ycfcpht8gmu85w3xyxzf

Republish once and exit:

  > ipfs name follow --once k51qzi5uqu5djwbl5v5r19lpxai8gxg0mtpgomn7s1ycfcpht8gmu85w3xyxzf
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The IPNS name to follow (e.g., PeerID or k51... key)."),
	},
	Options: []cmds.Option{
		cmds.StringOption(followIntervalOptionName, "i", "Interval between republishes.").WithDefault("30m"),
		cmds.BoolOption(followOnceOptionName, "Republish once and exit."),
		cmds.BoolOption(followVerboseOptionName, "v", "Print verbose progress information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		// Get command arguments and options
		keyArg := req.Arguments[0]
		intervalStr, _ := req.Options[followIntervalOptionName].(string)
		once, _ := req.Options[followOnceOptionName].(bool)
		verbose, _ := req.Options[followVerboseOptionName].(bool)

		// Parse the interval duration
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			return fmt.Errorf("invalid interval format: %w", err)
		}

		if interval < time.Minute && !once {
			return errors.New("interval must be at least 1 minute")
		}

		// Get the API instance
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// Log command start
		if verbose {
			fmt.Printf("[FOLLOW] Command started for key: %s\n", keyArg)
			fmt.Printf("[FOLLOW] Interval: %s, Once: %v\n", interval, once)
		}

		// Build the IPNS routing key
		ipnsKey := keyArg
		if len(ipnsKey) > 0 && ipnsKey[0] != '/' {
			ipnsKey = "/ipns/" + ipnsKey
		}

		// Parse and validate the IPNS name
		name, err := ipns.NameFromString(keyArg)
		if err != nil {
			return fmt.Errorf("invalid IPNS name: %w", err)
		}

		routingKey := "/ipns/" + name.String()

		if verbose {
			fmt.Printf("[FOLLOW] Parsed IPNS name: %s\n", name.String())
			fmt.Printf("[FOLLOW] Routing key: %s\n", routingKey)
		}

		// Fetch the record from the DHT
		if verbose {
			fmt.Printf("[FOLLOW] Fetching record from DHT...\n")
		}

		recordBytes, err := api.Routing().Get(req.Context, routingKey)
		if err != nil {
			errMsg := fmt.Sprintf("failed to fetch IPNS record from DHT: %v", err)
			if verbose {
				fmt.Printf("[FOLLOW] Error: %s\n", errMsg)
			}
			return errors.New(errMsg)
		}

		if verbose {
			fmt.Printf("[FOLLOW] Record found in DHT, size: %d bytes\n", len(recordBytes))
		}

		// Unmarshal record for validation (do not modify!)
		record, err := ipns.UnmarshalRecord(recordBytes)
		if err != nil {
			return fmt.Errorf("failed to parse IPNS record: %w", err)
		}

		// Check record validity (expiration time)
		validity, err := record.Validity()
		if err != nil {
			return fmt.Errorf("failed to get record validity: %w", err)
		}

		if verbose {
			fmt.Printf("[FOLLOW] Record validity: %s\n", validity.Format(time.RFC3339))
		}

		if validity.Before(time.Now()) {
			return fmt.Errorf("IPNS record has expired at %s", validity.Format(time.RFC3339))
		}

		// Get record value for logging
		value, err := record.Value()
		if err == nil && verbose {
			fmt.Printf("[FOLLOW] Record points to: %s\n", value.String())
		}

		// Create the republish function
		republish := func(ctx context.Context) error {
			if verbose {
				fmt.Printf("[FOLLOW] Republishing now at %s...\n", time.Now().Format(time.RFC3339))
			}

			// Republish the record as-is (opaque bytes, no modification)
			err := api.Routing().Put(ctx, routingKey, recordBytes)
			if err != nil {
				if verbose {
					fmt.Printf("[FOLLOW] Error during republish: %v\n", err)
				}
				return err
			}

			if verbose {
				fmt.Printf("[FOLLOW] Success! Record republished to DHT\n")
			}

			return nil
		}

		// Single execution mode (--once flag)
		if once {
			err := republish(req.Context)
			if err != nil {
				return res.Emit(&FollowResult{
					Key:       name.String(),
					Status:    "error",
					Timestamp: time.Now().Format(time.RFC3339),
					Error:     err.Error(),
				})
			}
			return res.Emit(&FollowResult{
				Key:       name.String(),
				Status:    "republished",
				Timestamp: time.Now().Format(time.RFC3339),
			})
		}

		// Execute first iteration immediately
		err = republish(req.Context)
		if err != nil {
			// First error is not fatal, continue with the loop
			if verbose {
				fmt.Printf("[FOLLOW] Initial republish failed, will retry: %v\n", err)
			}
		}

		// Emit initial status
		if err := res.Emit(&FollowResult{
			Key:       name.String(),
			Status:    "started",
			Timestamp: time.Now().Format(time.RFC3339),
		}); err != nil {
			return err
		}

		// Start the background worker for periodic republishing
		if verbose {
			fmt.Printf("[FOLLOW] Starting background worker with interval %s\n", interval)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check record validity before each republish
				if validity.Before(time.Now()) {
					errMsg := fmt.Sprintf("IPNS record expired at %s, stopping follow", validity.Format(time.RFC3339))
					if verbose {
						fmt.Printf("[FOLLOW] %s\n", errMsg)
					}
					return res.Emit(&FollowResult{
						Key:       name.String(),
						Status:    "expired",
						Timestamp: time.Now().Format(time.RFC3339),
						Error:     errMsg,
					})
				}

				err := republish(req.Context)
				status := "republished"
				errStr := ""
				if err != nil {
					status = "error"
					errStr = err.Error()
				}

				if err := res.Emit(&FollowResult{
					Key:       name.String(),
					Status:    status,
					Timestamp: time.Now().Format(time.RFC3339),
					Error:     errStr,
				}); err != nil {
					return err
				}

			case <-req.Context.Done():
				if verbose {
					fmt.Printf("[FOLLOW] Context cancelled, stopping\n")
				}
				return res.Emit(&FollowResult{
					Key:       name.String(),
					Status:    "stopped",
					Timestamp: time.Now().Format(time.RFC3339),
				})
			}
		}
	},
	Type: FollowResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *FollowResult) error {
			if out.Error != "" {
				_, err := fmt.Fprintf(w, "[%s] %s: %s (error: %s)\n", out.Timestamp, out.Key, out.Status, out.Error)
				return err
			}
			_, err := fmt.Fprintf(w, "[%s] %s: %s\n", out.Timestamp, out.Key, out.Status)
			return err
		}),
	},
}
