package prerequisites

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func createProcessImprovementBranch(cCtx *cli.Context) error {
	fmt.Println("Create process improvement branch")
	return nil
}

var CreateProcessImprovementBranch = &cli.Command{
	Name:   "create-process-improvement-branch",
	Action: createProcessImprovementBranch,
}
