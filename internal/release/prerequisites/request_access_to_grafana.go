package prerequisites

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func requestAccessToGrafana(cCtx *cli.Context) error {
	fmt.Println("Request access to grafana")
	return nil
}

var RequestAccessToGrafana = &cli.Command{
	Name:   "request-access-to-grafana",
	Action: requestAccessToGrafana,
}
