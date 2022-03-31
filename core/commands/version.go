package commands

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/core/commands/cmdenv"

	cmds "github.com/ipfs/go-ipfs-cmds"

	versioncmp "github.com/hashicorp/go-version"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	pstore "github.com/libp2p/go-libp2p/core/peerstore"
)

const (
	versionNumberOptionName             = "number"
	versionCommitOptionName             = "commit"
	versionRepoOptionName               = "repo"
	versionAllOptionName                = "all"
	versionCompareNewFractionOptionName = "--newer-fraction"
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

type CheckOutput struct {
	PeersCounted    int
	GreatestVersion string
	OldVersion      bool
}

var checkVersionCommand = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Checks IPFS version against network (online only).",
		ShortDescription: `
Checks node versions in our DHT to compare if we're running an older version.`,
	},
	Options: []cmds.Option{
		cmds.FloatOption(versionCompareNewFractionOptionName, "f", "Fraction of peers with new version to generate update warning.").WithDefault(0.1),
	},
	Type: CheckOutput{},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		if nd.DHT == nil {
			return ErrNotDHT
		}

		ourVersion, err := versioncmp.NewVersion(strings.Replace(version.CurrentVersionNumber, "-dev", "", -1))
		if err != nil {
			return fmt.Errorf("could not parse our own version %s: %w",
				version.CurrentVersionNumber, err)
		}

		greatestVersionSeen := ourVersion
		totalPeersCounted := 1 // Us (and to avoid division-by-zero edge case).
		withGreaterVersion := 0

		recordPeerVersion := func(agentVersion string) {
			// We process the version as is it assembled in GetUserAgentVersion.
			segments := strings.Split(agentVersion, "/")
			if len(segments) < 2 {
				return
			}
			if segments[0] != "kubo" {
				return
			}
			versionNumber := segments[1] // As in our CurrentVersionNumber.

			// Ignore development releases.
			if strings.Contains(versionNumber, "-dev") {
				return
			}
			if strings.Contains(versionNumber, "-rc") {
				return
			}

			peerVersion, err := versioncmp.NewVersion(versionNumber)
			if err != nil {
				// Do not error on invalid remote versions, just ignore.
				return
			}

			// Valid peer version number.
			totalPeersCounted += 1
			if ourVersion.LessThan(peerVersion) {
				withGreaterVersion += 1
			}
			if peerVersion.GreaterThan(greatestVersionSeen) {
				greatestVersionSeen = peerVersion
			}
		}

		// Logic taken from `ipfs stats dht` command.
		if nd.DHTClient != nd.DHT {
			client, ok := nd.DHTClient.(*fullrt.FullRT)
			if !ok {
				return cmds.Errorf(cmds.ErrClient, "could not generate stats for the WAN DHT client type")
			}
			for _, p := range client.Stat() {
				if ver, err := nd.Peerstore.Get(p, "AgentVersion"); err == nil {
					recordPeerVersion(ver.(string))
				} else if err == pstore.ErrNotFound {
					// ignore
				} else {
					// this is a bug, usually.
					log.Errorw(
						"failed to get agent version from peerstore",
						"error", err,
					)
				}
			}
		} else {
			for _, pi := range nd.DHT.WAN.RoutingTable().GetPeerInfos() {
				if ver, err := nd.Peerstore.Get(pi.Id, "AgentVersion"); err == nil {
					recordPeerVersion(ver.(string))
				} else if err == pstore.ErrNotFound {
					// ignore
				} else {
					// this is a bug, usually.
					log.Errorw(
						"failed to get agent version from peerstore",
						"error", err,
					)
				}
			}
		}

		newerFraction, _ := req.Options[versionCompareNewFractionOptionName].(float64)
		if err := cmds.EmitOnce(res, CheckOutput{
			PeersCounted:    totalPeersCounted,
			GreatestVersion: greatestVersionSeen.String(),
			OldVersion:      (float64(withGreaterVersion) / float64(totalPeersCounted)) > newerFraction,
		}); err != nil {
			return err
		}
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, checkOutput CheckOutput) error {
			if checkOutput.OldVersion {
				fmt.Fprintf(w, "⚠️WARNING: this Kubo node is running an outdated version compared to other peers, update to %s\n", checkOutput.GreatestVersion)
			}
			return nil
		}),
	},
}
