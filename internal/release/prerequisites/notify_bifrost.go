package prerequisites

import (
	"context"

	"github.com/ipfs/kubo/internal/github"
	rest "github.com/google/go-github/v48/github"
	"github.com/urfave/cli/v2"
)

func notifyBifrost(cCtx *cli.Context) error {
	gc, err := github.NewClient()
	if (err != nil) {
		return err
	}
	title := "Rollout Kubo v0.17.0-RC1 to a Cluster, Gateway and Bootstrapper bank"
	body := `@protocol/bifrost-team, we are going to release a new Kubo version on Nov-09.

We'll update this issue and ping in the bifrost Slack channel when the RC is done.

Thanks!`
	_, _, err = gc.Rest.Issues.Create(context.Background(), "protocol", "bifrost-infra", &rest.IssueRequest{
		Title: &title,
		Body: &body,
	})
	if (err != nil) {
		return err
	}
	return nil
}

var NotifyBifrost = &cli.Command{
	Name:   "notify-bifrost",
	Action: notifyBifrost,
}
