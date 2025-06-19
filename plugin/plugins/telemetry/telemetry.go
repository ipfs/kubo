package telemetry

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/repo"
	golibp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/pnet"
	"go.uber.org/fx"
	"golang.org/x/term"
)

var log = logging.Logger("telemetry")

const (
	envVar   = "IPFS_TELEMETRY_PLUGIN_MODE"
	filename = "telemetry_mode"
)

type pluginMode int

const (
	modeAsk pluginMode = iota
	modeOptIn
	modeOptOut
)

// Plugins sets the list of plugins to be loaded.
var Plugins = []plugin.Plugin{
	&telemetryPlugin{},
}

// telemetryPlugin is an FX plugin for Kubo that detects if the node is running on a private network.
type telemetryPlugin struct {
	filename string
	mode     pluginMode
	pnet     bool
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
		p.mode = modeOptOut
	}

	repoPath := env.Repo
	p.filename = path.Join(repoPath, filename)

	v := os.Getenv(envVar)
	if v != "" {
		log.Debug("mode set from env-var")
	} else { // try with file
		b, err := os.ReadFile(p.filename)
		if err == nil {
			v = string(b)
			v = strings.TrimSpace(v)
			log.Debug("mode set from file")
		} else if cfg := env.Config; cfg != nil { // try with config
			fmt.Printf("cfg not nil: %+v", cfg)
			pcfg, ok := cfg.(map[string]interface{})
			if ok {
				pmode, ok := pcfg["Mode"].(string)
				if ok {
					v = pmode
					log.Debug("mode set from config")
				}
			}
		}
	}

	log.Debug("telemetry mode: ", v)

	switch v {
	case "optin":
		p.mode = modeOptIn
	case "optout":
		p.mode = modeOptOut
	default:
		p.mode = modeAsk
	}

	return nil
}

func (p *telemetryPlugin) setupTelemetry(cfg *config.Config, repo repo.Repo) {
	// if we have no host, then we won't be able
	// to send telemetry anyways.
	if p.mode == modeOptOut {
		return
	}

	// Check whether we have a standard setup.
	standardSetup := true
	if pnet.ForcePrivateNetwork {
		standardSetup = false
	} else if key, _ := repo.SwarmKey(); key != nil {
		standardSetup = false
	} else if !hasDefaultBootstrapPeers(cfg) {
		standardSetup = false
	}

	// in a standard setup we don't ask, assume opt-in. This does not
	// change the status-quo as tracking already exist via
	// bootstrappers/peerlog. We are just officializing it under a
	// separate telemetry-protocol.
	if standardSetup {
		p.mode = modeOptIn
		return
	}

	// on non-standard setups, we ask

	// user gave permission already
	if p.mode == modeOptIn {
		p.pnet = true
		return
	}

	// ask and send if allowed.
	if p.pnetPromptWithTimeout(15 * time.Second) {
		p.pnet = true
		return
	}
}

// ParamsIn are the params for the decorator. Includes golibp2p.Option so that
// it is called before initializing a Host.
type ParamsIn struct {
	fx.In
	Repo repo.Repo
	Cfg  *config.Config
	Opts [][]golibp2p.Option `group:"libp2p"`
}

// ParamsOut includes the modified Opts from the decorator.
type ParamsOut struct {
	fx.Out
	Opts [][]golibp2p.Option `group:"libp2p"`
}

// Decorator hijacks the initialization process. Params ensures that we are
// called before the libp2p Host is initialized and starts listening, as we declare that we want to return a Libp2p Option (even if we don't).
func (p *telemetryPlugin) Decorator(in ParamsIn) (out ParamsOut) {
	log.Debug("telemetry decorator executed")
	p.setupTelemetry(in.Cfg, in.Repo)
	// out.Opts = append(in.Opts, []golibp2p.Option{golibp2p.UserAgent("my ass")})
	out.Opts = in.Opts
	return
}

type TelemetryIn struct {
	fx.In

	Host host.Host `optional:"true"`
}

func (p *telemetryPlugin) Telemetry(in TelemetryIn) {
	if p.mode == modeOptOut || in.Host == nil {
		return
	}

	if p.pnet {
		sendPnetTelemetry(in.Host)
		return
	}
	sendTelemetry(in.Host)
}

func (p *telemetryPlugin) Options(info core.FXNodeInfo) ([]fx.Option, error) {
	if p.mode == modeOptOut {
		return info.FXOptions, nil
	}

	opts := append(
		info.FXOptions,
		fx.Decorate(p.Decorator), //  runs pre Host creation
		fx.Invoke(p.Telemetry),   // runs post Host creation
	)
	return opts, nil
}

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

func (p *telemetryPlugin) pnetPromptWithTimeout(timeout time.Duration) bool {
	fmt.Print(`

*********************************************
*********************************************
ATTENTION: IT SEEMS YOU ARE RUNNING KUBO ON:

  * A PRIVATE NETWORK, or using
  * NON-STANDARD configuration (custom bootstrappers, amino DHT protocols)

The Kubo team is interested in learning more about how IPFS is used in private
networks. For example, we would like to understand if features like "private
networks" are used or can be removed in future releases.

Would you like to OPT-IN to send some anonymized telemetry to help us understand
your usage? If you OPT-IN:
  * A stable but temporary peer ID will be generated
  * The peer will bootstrap to our public bootstrappers
  * Anonymized metrics will be sent via the Telemetry protocol on every boot
  * No IP logging will happen
  * The temporary peer will disconnect aftewards
  * Telemetry can be controlled any time by setting:
    * Setting ` + envVar + ` to:
      * "ask" : Asks on every daemon start (default)
      * "optin": Sends telemetry on every daemon start.
      * "optout": Does not send telemetry and disables this message.
    * Creating $IPFS_HOME/` + filename + ` with the "ask", "optin", "optout" contents per the above.

IF YOU WOULD LIKE TO OPT-IN to telemetry, PRESS "Y".

IF YOU WOULD LIKE TO OPT-OUT to telemetry, PRESS "N".

(boot will continue in 15s)

Your answer (Y/N): `)

	pr, pw, err := os.Pipe()
	if err != nil {
		log.Error(err)
		return false
	}

	// We want to read a single key press without waiting
	// for the user to press enter.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return false
	}

	// nolint:errcheck
	go io.CopyN(pw, os.Stdin, 1)

	// Pipes support ReadDeadlines, so it seems nicer to use that
	// than signaling over a channel.
	err = pr.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		// nolint:errcheck
		term.Restore(int(os.Stdin.Fd()), oldState)
		return false
	}

	// Attempt to read one byte from the pipe
	b := make([]byte, 1)
	_, err = pr.Read(b)

	// nolint:errcheck
	term.Restore(int(os.Stdin.Fd()), oldState)

	// We timed out.
	// The following section is the only way to
	// achieve waiting for user input until a time-out happens.
	//
	// Read() is a blocking operation and there is NO WAY to cancel it.
	// We cannot close Stdin. We cannot write to Stdin. We can continue on
	// timeout but the reader on Stdin will remain blocked and we will
	// at least leak a goroutine until some input comes it.
	//
	// So the only way out of this is to Exec() ourselves again and
	// replace our running copy with a new one.
	// TODO: Test with supported architectures.
	if errors.Is(err, os.ErrDeadlineExceeded) {
		fmt.Printf("(Timed-out. Answer: Ask later)\n\n\n")

		time.Sleep(2 * time.Second)

		exec, err := os.Executable()
		if err != nil {
			return false
		}

		// make sure we do not re-execute the checks
		os.Setenv(envVar, "optout")
		env := os.Environ()

		// js/wasm doesn't have this. I hope reasonable architectures
		// have this.
		err = syscall.Exec(exec, os.Args, env)
		if err != nil {
			log.Error(err)
		}
		return false
	}

	// We didn't timeout. We errored in some other way or we actually
	// got user input.
	input := string(b[0])
	fmt.Println(input)

	// Close pipes.
	pr.Close()
	pw.Close()

	switch input {
	case "y", "Y":
		err = os.WriteFile(p.filename, []byte("optin"), 0600)
		if err != nil {
			log.Errorf("error saving telemetry preferences: %s", err)
		}
		return true
	case "n", "N":
		err = os.WriteFile(p.filename, []byte("optout"), 0600)
		if err != nil {
			log.Errorf("error saving telemetry preferences: %s", err)
		}

		return false
	default:
		return false
	}
}

func sendTelemetry(h host.Host) {
	fmt.Println("Sending Telemetry (TODO)", h.ID())
}

func sendPnetTelemetry(h host.Host) {
	fmt.Println("Sending pnet Telemetry (TODO)", h.ID())
}
