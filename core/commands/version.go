package commands

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	versioncmp "github.com/hashicorp/go-version"
	cmds "github.com/ipfs/go-ipfs-cmds"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	peer "github.com/libp2p/go-libp2p/core/peer"
	pstore "github.com/libp2p/go-libp2p/core/peerstore"
)

const (
	versionNumberOptionName         = "number"
	versionCommitOptionName         = "commit"
	versionRepoOptionName           = "repo"
	versionAllOptionName            = "all"
	versionCheckThresholdOptionName = "min-percent"
)

var VersionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Show IPFS version information.",
		ShortDescription: "Returns the current version of IPFS and exits.",
	},
	Subcommands: map[string]*cmds.Command{
		"deps":  depsVersionCommand,
		"check": checkVersionCommand,
	},

	Options: []cmds.Option{
		cmds.BoolOption(versionNumberOptionName, "n", "Only show the version number."),
		cmds.BoolOption(versionCommitOptionName, "Show the commit hash."),
		cmds.BoolOption(versionRepoOptionName, "Show repo version."),
		cmds.BoolOption(versionAllOptionName, "Show all version information"),
	},
	// must be permitted to run before init
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true), SetDoesNotUseConfigAsInput(true)),
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		return cmds.EmitOnce(res, version.GetVersionInfo())
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, version *version.VersionInfo) error {
			all, _ := req.Options[versionAllOptionName].(bool)
			if all {
				ver := version.Version
				if version.Commit != "" {
					ver += "-" + version.Commit
				}
				out := fmt.Sprintf("Kubo version: %s\n"+
					"Repo version: %s\nSystem version: %s\nGolang version: %s\n",
					ver, version.Repo, version.System, version.Golang)
				fmt.Fprint(w, out)
				return nil
			}

			commit, _ := req.Options[versionCommitOptionName].(bool)
			commitTxt := ""
			if commit && version.Commit != "" {
				commitTxt = "-" + version.Commit
			}

			repo, _ := req.Options[versionRepoOptionName].(bool)
			if repo {
				fmt.Fprintln(w, version.Repo)
				return nil
			}

			number, _ := req.Options[versionNumberOptionName].(bool)
			if number {
				fmt.Fprintln(w, version.Version+commitTxt)
				return nil
			}

			fmt.Fprintf(w, "ipfs version %s%s\n", version.Version, commitTxt)
			return nil
		}),
	},
	Type: version.VersionInfo{},
}

type Dependency struct {
	Path       string
	Version    string
	ReplacedBy string
	Sum        string
}

const pkgVersionFmt = "%s@%s"

var depsVersionCommand = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Shows information about dependencies used for build.",
		ShortDescription: `
Print out all dependencies and their versions.`,
	},
	Type: Dependency{},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return errors.New("no embedded dependency information")
		}
		toDependency := func(mod *debug.Module) (dep Dependency) {
			dep.Path = mod.Path
			dep.Version = mod.Version
			dep.Sum = mod.Sum
			if repl := mod.Replace; repl != nil {
				dep.ReplacedBy = fmt.Sprintf(pkgVersionFmt, repl.Path, repl.Version)
			}
			return
		}
		if err := res.Emit(toDependency(&info.Main)); err != nil {
			return err
		}
		for _, dep := range info.Deps {
			if err := res.Emit(toDependency(dep)); err != nil {
				return err
			}
		}
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, dep Dependency) error {
			fmt.Fprintf(w, pkgVersionFmt, dep.Path, dep.Version)
			if dep.ReplacedBy != "" {
				fmt.Fprintf(w, " => %s", dep.ReplacedBy)
			}
			fmt.Fprintf(w, "\n")
			return nil
		}),
	},
}

const DefaultMinimalVersionFraction = 0.05 // 5%

type VersionCheckOutput struct {
	UpdateAvailable    bool
	RunningVersion     string
	GreatestVersion    string
	PeersSampled       int
	WithGreaterVersion int
}

var checkVersionCommand = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Checks Kubo version against connected peers.",
		ShortDescription: `
This command uses the libp2p identify protocol to check the 'AgentVersion'
of connected peers and see if the Kubo version we're running is outdated.

Peers with an AgentVersion that doesn't start with 'kubo/' are ignored.
'UpdateAvailable' is set to true only if the 'min-fraction' criteria are met.

The 'ipfs daemon' does the same check regularly and logs when a new version
is available. You can stop these regular checks by setting
Version.SwarmCheckEnabled:false in the config.
`,
	},
	Options: []cmds.Option{
		cmds.IntOption(versionCheckThresholdOptionName, "t", "Percentage (1-100) of sampled peers with the new Kubo version needed to trigger an update warning.").WithDefault(config.DefaultSwarmCheckPercentThreshold),
	},
	Type: VersionCheckOutput{},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		minPercent, _ := req.Options[versionCheckThresholdOptionName].(int64)
		output, err := DetectNewKuboVersion(nd, minPercent)
		if err != nil {
			return err
		}

		if err := cmds.EmitOnce(res, output); err != nil {
			return err
		}
		return nil
	},
}

// DetectNewKuboVersion observers kubo version reported by other peers via
// libp2p identify protocol and notifies when threshold fraction of seen swarm
// is running updated Kubo. It is used by RPC and CLI at 'ipfs version check'
// and also periodically when 'ipfs daemon' is running.
func DetectNewKuboVersion(nd *core.IpfsNode, minPercent int64) (VersionCheckOutput, error) {
	ourVersion, err := versioncmp.NewVersion(version.CurrentVersionNumber)
	if err != nil {
		return VersionCheckOutput{}, fmt.Errorf("could not parse our own version %q: %w",
			version.CurrentVersionNumber, err)
	}
	// MAJOR.MINOR.PATCH without any suffix
	ourVersion = ourVersion.Core()

	greatestVersionSeen := ourVersion
	totalPeersSampled := 1 // Us (and to avoid division-by-zero edge case)
	withGreaterVersion := 0

	recordPeerVersion := func(agentVersion string) {
		// We process the version as is it assembled in GetUserAgentVersion
		segments := strings.Split(agentVersion, "/")
		if len(segments) < 2 {
			return
		}
		if segments[0] != "kubo" {
			return
		}
		versionNumber := segments[1] // As in our CurrentVersionNumber

		peerVersion, err := versioncmp.NewVersion(versionNumber)
		if err != nil {
			// Do not error on invalid remote versions, just ignore
			return
		}

		// Ignore prereleases and development releases (-dev, -rcX)
		if peerVersion.Metadata() != "" || peerVersion.Prerelease() != "" {
			return
		}

		// MAJOR.MINOR.PATCH without any suffix
		peerVersion = peerVersion.Core()

		// Valid peer version number
		totalPeersSampled += 1
		if ourVersion.LessThan(peerVersion) {
			withGreaterVersion += 1
		}
		if peerVersion.GreaterThan(greatestVersionSeen) {
			greatestVersionSeen = peerVersion
		}
	}

	processPeerstoreEntry := func(id peer.ID) {
		if v, err := nd.Peerstore.Get(id, "AgentVersion"); err == nil {
			recordPeerVersion(v.(string))
		} else if errors.Is(err, pstore.ErrNotFound) { // ignore noop
		} else { // a bug, usually.
			log.Errorw("failed to get agent version from peerstore", "error", err)
		}
	}

	// Amino DHT client keeps information about previously seen peers
	if nd.DHTClient != nd.DHT && nd.DHTClient != nil {
		client, ok := nd.DHTClient.(*fullrt.FullRT)
		if !ok {
			return VersionCheckOutput{}, errors.New("could not perform version check due to missing or incompatible DHT configuration")
		}
		for _, p := range client.Stat() {
			processPeerstoreEntry(p)
		}
	} else if nd.DHT != nil && nd.DHT.WAN != nil {
		for _, pi := range nd.DHT.WAN.RoutingTable().GetPeerInfos() {
			processPeerstoreEntry(pi.Id)
		}
	} else if nd.DHT != nil && nd.DHT.LAN != nil {
		for _, pi := range nd.DHT.LAN.RoutingTable().GetPeerInfos() {
			processPeerstoreEntry(pi.Id)
		}
	} else {
		return VersionCheckOutput{}, errors.New("could not perform version check due to missing or incompatible DHT configuration")
	}

	if minPercent < 1 || minPercent > 100 {
		if minPercent == 0 {
			minPercent = config.DefaultSwarmCheckPercentThreshold
		} else {
			return VersionCheckOutput{}, errors.New("Version.SwarmCheckPercentThreshold must be between 1 and 100")
		}
	}

	minFraction := float64(minPercent) / 100.0

	// UpdateAvailable flag is set only if minFraction was reached
	greaterFraction := float64(withGreaterVersion) / float64(totalPeersSampled)

	// Gathered metric are returned every time
	return VersionCheckOutput{
		UpdateAvailable:    (greaterFraction >= minFraction),
		RunningVersion:     ourVersion.String(),
		GreatestVersion:    greatestVersionSeen.String(),
		PeersSampled:       totalPeersSampled,
		WithGreaterVersion: withGreaterVersion,
	}, nil
}
