package kubo

import (
	"context"
	"fmt"
	"os"

	"dagger.io/dagger"
	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
)

type ProcessImprovement mg.Namespace

func getProcessImprovementBranchName(version string) string {
	return "process-improvement-" + version
}

func getProcessImprovementPRTitle(version string) string {
	return "Process Improvement: " + version
}

func getProcessImprovementPRBody(version, url string) string {
	return fmt.Sprintf(
		`Related to %s

This PR is to track the process improvement as identified during the %s release.`,
		url, version)
}

func (ProcessImprovement) CreateProcessImprovementPR(ctx context.Context, version string) error {
	head := getProcessImprovementBranchName(version)

	pr, err := util.GetPR(ctx, Owner, Repo, head)
	if err != nil {
		return err
	}
	if pr != nil {
		fmt.Printf("PR already exists: %s", pr.GetHTMLURL())
		return nil
	}

	branch, err := util.GetBranch(ctx, Owner, Repo, head)
	if err != nil {
		return err
	}
	if branch == nil {
		branch, err = util.CreateBranch(ctx, Owner, Repo, head, DefaultBranchName)
		if err != nil {
			return err
		}
	}

	ki, err := GetIssue(ctx, version)
	if err != nil {
		return err
	}

	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer c.Close()

	container := c.Container().From("alpine:3.14.2")
	container = util.WithGit(container)
	container = util.WithCheckout(container, Owner, Repo, branch.GetName(), branch.GetCommit().GetSHA())
	container = container.WithExec([]string{"sed", "-i", "1s;.*;<!-- Last updated during [" + version + " release](" + ki.GetHTMLURL() + ") -->;", "docs/RELEASE_ISSUE_TEMPLATE.md"})
	container = container.WithExec([]string{"git", "diff", "--name-only", "docs/RELEASE_ISSUE_TEMPLATE.md"})

	diff, err := container.Stdout(ctx)
	if err != nil {
		return err
	}
	if diff != "" {
		fmt.Println("Updating docs/RELEASE_ISSUE_TEMPLATE.md")
		container = container.WithExec([]string{"git", "add", "docs/RELEASE_ISSUE_TEMPLATE.md"})
		container = container.WithExec([]string{"git", "commit", "-m", "'docs: update RELEASE_ISSUE_TEMPLATE.md'"})
		container = container.WithExec([]string{"git", "push", "origin", head})

		stderr, err := container.Stderr(ctx)
		if err != nil {
			fmt.Println(stderr)
			return err
		}
	}

	title := getProcessImprovementPRTitle(version)
	body := getProcessImprovementPRBody(version, ki.GetHTMLURL())

	pr, err = util.CreatePR(ctx, Owner, Repo, head, DefaultBranchName, title, body, true)
	if err != nil {
		return err
	}

	fmt.Printf("PR created: %s", pr.GetHTMLURL())
	return nil
}
