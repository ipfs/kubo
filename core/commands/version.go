package commands

import (
	"fmt"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	config "github.com/ipfs/go-ipfs/repo/config"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
)

type VersionOutput struct {
	Version string
	Commit  string
	Full    string
	Repo    string
}

var VersionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Shows ipfs version information.",
		ShortDescription: "Returns the current version of ipfs and exits.",
	},

	Options: []cmds.Option{
		cmds.BoolOption("number", "n", "Only show the version number.").Default(false),
		cmds.BoolOption("commit", "Show the commit hash. Deprecated.").Default(false),
		cmds.BoolOption("full", "Show full version based on git tag & commit.").Default(false),
		cmds.BoolOption("repo", "Show repo version.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		res.SetOutput(&VersionOutput{
			Version: config.CurrentVersionNumber,
			Commit:  config.CurrentCommit,
			Full:    config.FullVersion,
			Repo:    fsrepo.RepoVersion,
		})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v := res.Output().(*VersionOutput)

			repo, _, err := res.Request().Option("repo").Bool()
			if err != nil {
				return nil, err
			}

			if repo {
				return strings.NewReader(v.Repo + "\n"), nil
			}

			full, _, err := res.Request().Option("full").Bool()
			if err != nil {
				return nil, err
			}
			commit, _, err := res.Request().Option("commit").Bool()
			if err != nil {
				return nil, err
			}

			commitTxt := ""
			if full {
				commitTxt = v.Full
			} else {
				if commit {
					commitTxt = v.Version + "-" + v.Commit
				} else {
					commitTxt = v.Version
				}
			}

			number, _, err := res.Request().Option("number").Bool()
			if err != nil {
				return nil, err
			}
			if number {
				return strings.NewReader(fmt.Sprintln(commitTxt)), nil
			}

			return strings.NewReader(fmt.Sprintf("ipfs version %s\n", commitTxt)), nil
		},
	},
	Type: VersionOutput{},
}
