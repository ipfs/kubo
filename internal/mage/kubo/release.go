package kubo

import (
	"context"
	"fmt"
	"os"
	"strings"

	"dagger.io/dagger"
	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"golang.org/x/mod/semver"
)

type Release mg.Namespace

func getReleaseBranchName(version string) string {
	return "release-" + semver.MajorMinor(version)
}

func getReleasePRTitle(version string) string {
	return "Release: " + semver.MajorMinor(version)
}

func getReleasePRBody(version, url string) string {
	return fmt.Sprintf(
		`Related to %s

This PR is to track the %s release.`,
		url, semver.MajorMinor(version))
}

func (Release) CutReleaseBranch(ctx context.Context, version string) error {
	head := getReleaseBranchName(version)

	branch, err := util.GetBranch(ctx, Owner, Repo, head)
	if err != nil {
		return err
	}
	if branch != nil {
		fmt.Println("Branch already exists")
		return nil
	}

	_, err = util.CreateBranch(ctx, Owner, Repo, head, DefaultBranchName)
	if err != nil {
		return err
	}

	fmt.Println("Branch created")
	return nil
}

func (Release) UpdateReleaseVersion(ctx context.Context, version string) error {
	head := getReleaseBranchName(version)

	branch, err := util.GetBranch(ctx, Owner, Repo, head)
	if err != nil {
		return err
	}
	if branch == nil {
		return fmt.Errorf("branch %s does not exist", head)
	}

	f, err := util.GetFile(ctx, Owner, Repo, "version.go", head)
	if err != nil {
		return err
	}

	if strings.Contains(*f.Content, version) {
		fmt.Println("Release version already updated")
		return nil
	}

	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer c.Close()

	container := c.Container().From("alpine:3.14.2")
	container = util.WithGit(container)
	container = util.WithCheckout(container, Owner, Repo, branch.GetName(), branch.GetCommit().GetSHA())
	container = container.WithExec([]string{"sed", "-i", "s;const CurrentVersionNumber = \".*\";const CurrentVersionNumber = \"" + version + "\";", "version.go"})

	container = container.WithExec([]string{"git", "add", "version.go"})
	container = container.WithExec([]string{"git", "commit", "-m", "chore: update version.go"})
	container = container.WithExec([]string{"git", "push", "origin", head})

	stderr, err := container.Stderr(ctx)
	if err != nil {
		fmt.Println(stderr)
		return err
	}

	fmt.Println("Release version updated")
	return nil
}

func (Release) CreateReleasePR(ctx context.Context, version string) error {
	head := getReleaseBranchName(version)

	branch, err := util.GetBranch(ctx, Owner, Repo, head)
	if err != nil {
		return err
	}
	if branch == nil {
		return fmt.Errorf("branch %s does not exist", head)
	}

	pr, err := util.GetPR(ctx, Owner, Repo, head)
	if err != nil {
		return err
	}
	if pr != nil {
		fmt.Printf("PR already exists: %s", pr.GetHTMLURL())
		return nil
	}

	ki, err := GetIssue(ctx, version)
	if err != nil {
		return err
	}

	title := getReleasePRTitle(version)
	body := getReleasePRBody(version, ki.GetHTMLURL())

	pr, err = util.CreatePR(ctx, Owner, Repo, head, ReleaseBranchName, title, body, true)
	if err != nil {
		return err
	}

	fmt.Printf("PR created: %s", pr.GetHTMLURL())
	return nil
}

func (Release) CheckCI(ctx context.Context, version string) error {
	head := getReleaseBranchName(version)

	runs, err := util.GetCheckRuns(ctx, Owner, Repo, head)
	if err != nil {
		return err
	}

	for _, run := range runs {
		if run.GetStatus() != "completed" {
			return fmt.Errorf("check %s is not completed", run.GetName())
		}
		if run.GetConclusion() != "success" {
			return fmt.Errorf("check %s is not successful", run.GetName())
		}
	}

	fmt.Println("All checks are successful")
	return nil
}
