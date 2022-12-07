package bifrost

import (
	"context"
	"fmt"

	"github.com/ipfs/kubo/internal/mage/kubo"
	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
)

type Issue mg.Namespace

const (
	owner = "protocol"
	repo  = "bifrost-infra"
)

func getIssueTitle(version string) string {
	return "Rollout Kubo " + version + " to a Cluster, Gateway and Bootstrapper bank"
}

func getIssueBody(version, url string) string {
	return fmt.Sprintf(
		`Related to %s

This issue is to track the rollout of Kubo %s to a Cluster, Gateway and Bootstrapper bank.`,
		url, version)
}

func getCommentBody(version string) string {
	return "TEST"
}

func (Issue) CreateIssue(ctx context.Context, version string) error {
	title := getIssueTitle(version)

	i, err := util.GetIssue(ctx, owner, repo, title)
	if err != nil {
		return err
	}
	if i != nil {
		fmt.Printf("Issue already exists: %s", i.GetHTMLURL())
		return nil
	}

	ki, err := kubo.GetIssue(ctx, version)
	if err != nil {
		return err
	}
	if ki == nil {
		return fmt.Errorf("kubo issue not found")
	}

	body := getIssueBody(version, ki.GetHTMLURL())

	i, err = util.CreateIssue(ctx, owner, repo, title, body)
	if err != nil {
		return err
	}

	fmt.Printf("Issue created: %s", i.GetHTMLURL())
	return nil
}

func (Issue) CreateIssueComment(ctx context.Context, version string) error {
	title := getIssueTitle(version)
	body := getCommentBody(version)

	comment, err := util.GetIssueComment(ctx, owner, repo, title, body)
	if err != nil {
		return err
	}

	if comment != nil {
		fmt.Printf("Comment already exists: %s", comment.GetHTMLURL())
		return nil
	}

	comment, err = util.CreateIssueComment(ctx, owner, repo, title, body)
	if err != nil {
		return err
	}

	fmt.Printf("Comment created: %s", comment.GetHTMLURL())
	return nil
}
