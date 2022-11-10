package github

import (
	"os"
	"fmt"
	"context"

	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"
)

type GitHubClient struct {
	Rest *github.Client
}

func NewClient() (*GitHubClient, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is not set")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return &GitHubClient{
		Rest: github.NewClient(tc),
	}, nil
}

/*

	issues, _, err := gc.Rest.Search.Issues(context.Background(), "repo:ipfs/kubo is:issue title:\"Release 0.17\"", nil)
	if (err != nil) {
		return err
	}
	if (len(issues.Issues) != 1) {
		return fmt.Errorf("expected to find 1 issue, found %d", len(issues.Issues))
	}
	issue := issues.Issues[0]
*/
