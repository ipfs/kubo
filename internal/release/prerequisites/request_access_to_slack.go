package prerequisites

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func requestAccessToSlack(cCtx *cli.Context) error {
	fmt.Println("Request access to slack")
	return nil
}

var RequestAccessToSlack = &cli.Command{
	Name:   "request-access-to-slack",
	Action: requestAccessToSlack,
}
