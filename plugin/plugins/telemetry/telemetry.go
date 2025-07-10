package telemetry

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/plugin"
)

var log = logging.Logger("telemetry")

const (
	modeEnvVar   = "IPFS_TELEMETRY_PLUGIN_MODE"
	uuidFilename = "telemetry_uuid"
	endpoint     = "http://127.0.0.1:8083"
)

type pluginMode int

const (
	modeInfo pluginMode = iota
	modeOptIn
	modeOptOut
)

type LogEvent struct {
	UUID string `json:"uuid"`

	AgentVersion string `json:"agent_version"`

	PrivateNetwork bool `json:"private_network"`

	BootstrappersCustom bool `json:"bootstrappers_custom"`

	RepoSizeBucket int `json:"repo_size_bucket"`

	UptimeBucket int `json:"uptime_bucket"`

	ReproviderStrategy string `json:"reprovider_strategy"`

	RoutingType                 string `json:"routing_type"`
	RoutingAcceleratedDHTClient bool   `json:"routing_accelerated_dht_client"`
	RoutingDelegatedCount       int    `json:"routing_delegated_count"`

	AutoNATServiceMode       string `json:"autonat_service_mode"`
	AutoNATPubliclyDiallable bool   `json:"autonat_publicly_diallable"`

	SwarmEnableHolePunching  bool `json:"swarm_enable_hole_punching"`
	SwarmRelayAddresses      bool `json:"swarm_relay_addresses"`
	SwarmIPv4PublicAddresses bool `json:"swarm_ipv4_public_addresses"`
	SwarmIPv6PublicAddresses bool `json:"swarm_ipv6_public_addresses"`

	AutoTLSAutoWSS            bool `json:"auto_tls_auto_wss"`
	AutoTLSDomainSuffixCustom bool `json:"auto_tls_domain_suffix_custom"`

	DiscoveryMDNSEnabled bool `json:"discovery_mdns_enabled"`

	PlatformOS            string `json:"platform_os"`
	PlatformArch          string `json:"platform_arch"`
	PlatformContainerized bool   `json:"platform_containerized"`
}

// Plugins sets the list of plugins to be loaded.
var Plugins = []plugin.Plugin{
	&telemetryPlugin{},
}

// telemetryPlugin is an FX plugin for Kubo that detects if the node is running on a private network.
type telemetryPlugin struct {
	uuidFilename string
	mode         pluginMode

	node  *core.IpfsNode
	event *LogEvent
}

func (p *telemetryPlugin) Name() string {
	return "telemetry"
}

func (p *telemetryPlugin) Version() string {
	return "0.0.1"
}

func (p *telemetryPlugin) Init(env *plugin.Environment) error {
	// logging.SetLogLevel("telemetry", "DEBUG")
	log.Debug("telemetry plugin Init()")
	// Whatever the setting, if we are not in daemon mode
	// or we an in offline mode, we auto-opt out.
	isDaemon := false
	isOffline := false
	for _, arg := range os.Args {
		if arg == "daemon" {
			isDaemon = true
		}
		if arg == "--offline" {
			isOffline = true
		}
	}
	if !isDaemon || isOffline {
		log.Debug("telemetry mode: optout (offline/one-shot)")
		p.mode = modeOptOut
		return nil

	}
	p.event = &LogEvent{}

	repoPath := env.Repo
	p.uuidFilename = path.Join(repoPath, uuidFilename)

	v := os.Getenv(modeEnvVar)
	if v != "" {
		log.Debug("mode set from env-var")
	} else if cfg := env.Config; cfg != nil { // try with config
		pcfg, ok := cfg.(map[string]interface{})
		if ok {
			pmode, ok := pcfg["Mode"].(string)
			if ok {
				v = pmode
				log.Debug("mode set from config")
			}
		}
	}

	log.Debug("telemetry mode: ", v)
	switch v {
	case "optout":
		p.mode = modeOptOut
		return nil
	case "info":
		p.mode = modeInfo
	default:
		p.mode = modeOptIn
	}

	// loadUUID might switch to modeInfo when generating a new uuid
	if err := p.loadUUID(); err != nil {
		p.mode = modeOptOut
		return nil
	}
	return nil
}

func (p *telemetryPlugin) loadUUID() error {
	// Generate or read our UUID from disk
	b, err := os.ReadFile(p.uuidFilename)
	if err == nil {
		v := string(b)
		v = strings.TrimSpace(v)
		uid, err := uuid.Parse(v)
		if err != nil {
			log.Errorf("cannot parse telemetry uuid: %s", err)
			return err
		}
		p.event.UUID = uid.String()
		return err
	} else if os.IsNotExist(err) {
		uid, err := uuid.NewRandom()
		if err != nil {
			log.Errorf("cannot generate telemetry uuid: %s", err)
			return err
		}
		p.event.UUID = uid.String()
		p.mode = modeInfo

		// Write the UUID to disk
		if err := os.WriteFile(p.uuidFilename, []byte(p.event.UUID), 0600); err != nil {
			log.Errorf("cannot write telemetry uuid: %s", err)
			return err
		}
	} else {
		log.Errorf("error reading telemetry uuid from disk: %s", err)
		return err
	}
	return nil
}

// func (p *telemetryPlugin) setupTelemetry(cfg *config.Config, repo repo.Repo) {
// 	// noop on opt out
// 	if p.mode == modeOptOut {
// 		return
// 	}

// 	// We should have a UUID
// 	if len(p.event.UUID) == 0 {
// 		return
// 	}

// 	if p.mode == modeInfo {
// 		p.showInfo()
// 	}

// 	// Check whether we have a standard setup.
// 	standardSetup := true
// 	if pnet.ForcePrivateNetwork {
// 		standardSetup = false
// 	} else if key, _ := repo.SwarmKey(); key != nil {
// 		standardSetup = false
// 	} else if !hasDefaultBootstrapPeers(cfg) {
// 		standardSetup = false
// 	}

// 	// in a standard setup we don't ask, assume opt-in. This does not
// 	// change the status-quo as tracking already exist via
// 	// bootstrappers/peerlog. We are just officializing it under a
// 	// separate telemetry-protocol.
// 	if standardSetup {
// 		p.mode = modeOptIn
// 		return
// 	}

// 	// on non-standard setups, we ask

// 	// user gave permission already
// 	if p.mode == modeOptIn {
// 		p.pnet = true
// 		return
// 	}

// 	// ask and send if allowed.
// 	if p.pnetPromptWithTimeout(15 * time.Second) {
// 		p.pnet = true
// 		return
// 	}
// }

// // ParamsIn are the params for the decorator. Includes golibp2p.Option so that
// // it is called before initializing a Host.
// type ParamsIn struct {
// 	fx.In
// 	Repo repo.Repo
// 	Cfg  *config.Config
// 	Opts [][]golibp2p.Option `group:"libp2p"`
// }

// // ParamsOut includes the modified Opts from the decorator.
// type ParamsOut struct {
// 	fx.Out
// 	Opts [][]golibp2p.Option `group:"libp2p"`
// }

// // Decorator hijacks the initialization process. Params ensures that we are
// // called before the libp2p Host is initialized and starts listening, as we declare that we want to return a Libp2p Option (even if we don't).
// func (p *telemetryPlugin) Decorator(in ParamsIn) (out ParamsOut) {
// 	log.Debug("telemetry decorator executed")
// 	p.setupTelemetry(in.Cfg, in.Repo)
// 	// out.Opts = append(in.Opts, []golibp2p.Option{golibp2p.UserAgent("my ass")})
// 	out.Opts = in.Opts
// 	return
// }

// type TelemetryIn struct {
// 	fx.In

// 	Host host.Host `optional:"true"`
// }

// func (p *telemetryPlugin) Telemetry(in TelemetryIn) {
// 	if p.mode == modeOptOut || in.Host == nil {
// 		return
// 	}

// 	if p.pnet {
// 		sendPnetTelemetry(in.Host)
// 		return
// 	}
// 	sendTelemetry(in.Host)
// }

// func (p *telemetryPlugin) Options(info core.FXNodeInfo) ([]fx.Option, error) {
// 	if p.mode == modeOptOut {
// 		return info.FXOptions, nil
// 	}

// 	opts := append(
// 		info.FXOptions,
// 		fx.Decorate(p.Decorator), //  runs pre Host creation
// 		fx.Invoke(p.Telemetry),   // runs post Host creation
// 	)
// 	return opts, nil
// }

func hasDefaultBootstrapPeers(cfg *config.Config) bool {
	defaultPeers := config.DefaultBootstrapAddresses
	currentPeers := cfg.Bootstrap
	if len(defaultPeers) != len(currentPeers) {
		return false
	}
	peerMap := make(map[string]bool)
	for _, peer := range defaultPeers {
		peerMap[peer] = true
	}
	for _, peer := range currentPeers {
		if !peerMap[peer] {
			return false
		}
	}
	return true
}

func (p *telemetryPlugin) showInfo() {
	fmt.Printf(`

** Telemetry enabled **

Kubo will send anonymized information to the developers:
  - Why?   The developers want to better understand the usage of different
           features.
  - What?  Anonymized information only. You can inspect it with
           'GOLOG_LOG_LEVEL="telemetry=debug"
  - When?  Soon after daemon's boot, or every 24h.
  - How?   An HTTP request to %s.

To opt-out, CTRL-C now and:
    * Set %s to "optput", or
    * Run 'ipfs config Plugins.Plugins.telemetry.Config.Mode optout'
    * This message will be shown only once.

Your telemetry UUID is: %s

`, endpoint, modeEnvVar, p.event.UUID)
}

func (p *telemetryPlugin) Start(n *core.IpfsNode) error {
	p.node = n
	if p.mode == modeOptOut {
		return nil
	}

	// We should have a UUID
	if len(p.event.UUID) == 0 {
		return nil
	}

	if p.mode == modeInfo {
		p.showInfo()
	}

	time.AfterFunc(time.Second, func() {

		sendTelemetry()
	})

	return errors.New("does this crash it?")
}

// func (p *telemetryPlugin) pnetPromptWithTimeout(timeout time.Duration) bool {
// 	fmt.Print(`

// *********************************************
// *********************************************
// ATTENTION: IT SEEMS YOU ARE RUNNING KUBO ON:

//   * A PRIVATE NETWORK, or using
//   * NON-STANDARD configuration (custom bootstrappers, amino DHT protocols)

// The Kubo team is interested in learning more about how IPFS is used in private
// networks. For example, we would like to understand if features like "private
// networks" are used or can be removed in future releases.

// Would you like to OPT-IN to send some anonymized telemetry to help us understand
// your usage? If you OPT-IN:
//   * A stable but temporary peer ID will be generated
//   * The peer will bootstrap to our public bootstrappers
//   * Anonymized metrics will be sent via the Telemetry protocol on every boot
//   * No IP logging will happen
//   * The temporary peer will disconnect aftewards
//   * Telemetry can be controlled any time by setting:
//     * Setting ` + envVar + ` to:
//       * "ask" : Asks on every daemon start (default)
//       * "optin": Sends telemetry on every daemon start.
//       * "optout": Does not send telemetry and disables this message.
//     * Creating $IPFS_HOME/` + filename + ` with the "ask", "optin", "optout" contents per the above.

// IF YOU WOULD LIKE TO OPT-IN to telemetry, PRESS "Y".

// IF YOU WOULD LIKE TO OPT-OUT to telemetry, PRESS "N".

// (boot will continue in 15s)

// Your answer (Y/N): `)

// 	pr, pw, err := os.Pipe()
// 	if err != nil {
// 		log.Error(err)
// 		return false
// 	}

// 	// We want to read a single key press without waiting
// 	// for the user to press enter.
// 	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
// 	if err != nil {
// 		return false
// 	}

// 	// nolint:errcheck
// 	go io.CopyN(pw, os.Stdin, 1)

// 	// Pipes support ReadDeadlines, so it seems nicer to use that
// 	// than signaling over a channel.
// 	err = pr.SetReadDeadline(time.Now().Add(timeout))
// 	if err != nil {
// 		// nolint:errcheck
// 		term.Restore(int(os.Stdin.Fd()), oldState)
// 		return false
// 	}

// 	// Attempt to read one byte from the pipe
// 	b := make([]byte, 1)
// 	_, err = pr.Read(b)

// 	// nolint:errcheck
// 	term.Restore(int(os.Stdin.Fd()), oldState)

// 	// We timed out.
// 	// The following section is the only way to
// 	// achieve waiting for user input until a time-out happens.
// 	//
// 	// Read() is a blocking operation and there is NO WAY to cancel it.
// 	// We cannot close Stdin. We cannot write to Stdin. We can continue on
// 	// timeout but the reader on Stdin will remain blocked and we will
// 	// at least leak a goroutine until some input comes it.
// 	//
// 	// So the only way out of this is to Exec() ourselves again and
// 	// replace our running copy with a new one.
// 	// TODO: Test with supported architectures.
// 	if errors.Is(err, os.ErrDeadlineExceeded) {
// 		fmt.Printf("(Timed-out. Answer: Ask later)\n\n\n")

// 		time.Sleep(2 * time.Second)

// 		exec, err := os.Executable()
// 		if err != nil {
// 			return false
// 		}

// 		// make sure we do not re-execute the checks
// 		os.Setenv(envVar, "optout")
// 		env := os.Environ()

// 		// js/wasm doesn't have this. I hope reasonable architectures
// 		// have this.
// 		err = syscall.Exec(exec, os.Args, env)
// 		if err != nil {
// 			log.Error(err)
// 		}
// 		return false
// 	}

// 	// We didn't timeout. We errored in some other way or we actually
// 	// got user input.
// 	input := string(b[0])
// 	fmt.Println(input)

// 	// Close pipes.
// 	pr.Close()
// 	pw.Close()

// 	switch input {
// 	case "y", "Y":
// 		err = os.WriteFile(p.filename, []byte("optin"), 0600)
// 		if err != nil {
// 			log.Errorf("error saving telemetry preferences: %s", err)
// 		}
// 		return true
// 	case "n", "N":
// 		err = os.WriteFile(p.filename, []byte("optout"), 0600)
// 		if err != nil {
// 			log.Errorf("error saving telemetry preferences: %s", err)
// 		}

// 		return false
// 	default:
// 		return false
// 	}
// }

func sendTelemetry() {
	fmt.Println("Sending Telemetry (TODO)")
}
