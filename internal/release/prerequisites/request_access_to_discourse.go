package prerequisites

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func requestAccessToDiscourse(cCtx *cli.Context) error {
	fmt.Println("Request access to discourse")
	return nil
}

var RequestAccessToDiscourse = &cli.Command{
	Name:   "request-access-to-discourse",
	Action: requestAccessToDiscourse,
}
