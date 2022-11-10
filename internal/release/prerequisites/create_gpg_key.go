package prerequisites

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func createGpgKey(cCtx *cli.Context) error {
	fmt.Println("Create GPG key")
	return nil
}

var CreateGpgKey = &cli.Command{
	Name:   "create-gpg-key",
	Action: createGpgKey,
}
