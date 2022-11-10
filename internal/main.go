package main

import (
	"log"
	"os"

	release_prerequisites "github.com/ipfs/kubo/internal/release/prerequisites"
	release_final "github.com/ipfs/kubo/internal/release/final"
	"github.com/urfave/cli/v2"
)


func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name: "release",
				Subcommands: []*cli.Command{
					{
						Name: "prerequisites",
						Subcommands: []*cli.Command{
							release_prerequisites.CreateChangelogProposal,
							release_prerequisites.CreateGpgKey,
							release_prerequisites.CreateProcessImprovementBranch,
							release_prerequisites.NotifyBifrost,
							release_prerequisites.RequestAccessToDiscourse,
							release_prerequisites.RequestAccessToGrafana,
							release_prerequisites.RequestAccessToSlack,
						},
					},
					{
						Name: "final",
						Subcommands: []*cli.Command{
							release_final.CreateReleaselog,
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
