package prerequisites

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func createChangelogProposal(cCtx *cli.Context) error {
	fmt.Println("Create changelog proposal")
	return nil
}

var CreateChangelogProposal = &cli.Command{
	Name:   "create-changelog-proposal",
	Action: createChangelogProposal,
}
