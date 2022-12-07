package kubo

import (
	"context"
	"fmt"

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

	title := getProcessImprovementPRTitle(version)
	body := getProcessImprovementPRBody(version, ki.GetHTMLURL())

	pr, err = util.CreatePR(ctx, Owner, Repo, head, DefaultBranchName, title, body, true)
	if err != nil {
		return err
	}

	fmt.Printf("PR created: %s", pr.GetHTMLURL())
	return nil
}
