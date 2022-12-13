package kubo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	sv "github.com/Masterminds/semver"
	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"golang.org/x/mod/semver"
)

type Release mg.Namespace

func getGitHubReleaseName(version string) string {
	return version
}

func getGitHubReleaseBody(version, url string) string {
	mm := semver.MajorMinor(version)
	rb := getReleaseBranchName(version)
	return fmt.Sprintf(
		`See the related issue: %s

And the draft changelog: [docs/changelogs/%s.md](https://github.com/ipfs/kubo/blob/%s/docs/changelogs/%s.md)`,
		url, mm, rb, mm)
}

func isGitHubReleasePrerelease(version string) (bool, error) {
	v, err := sv.NewVersion(version)
	if err != nil {
		return true, err
	}
	pre := v.Prerelease()
	return pre != "", nil
}

func getReleaseVersion(version string) (string, error) {
	v, err := sv.NewVersion(version)
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

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
	v, err := getReleaseVersion(version)
	if err != nil {
		return err
	}

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

	if strings.Contains(*f.Content, v) {
		fmt.Println("Release version already updated")
		return nil
	}

	dir, err := os.MkdirTemp("", "dist")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	err = util.GitClone(dir, Owner, Repo, branch.GetName(), branch.GetCommit().GetSHA())
	if err != nil {
		return err
	}

	cmd := exec.Command("sed", "-i", "s;const CurrentVersionNumber = \".*\";const CurrentVersionNumber = \""+v+"\";", "version.go")
	cmd.Dir = dir
	err = cmd.Run()
	if err != nil {
		return err
	}

	err = util.GitCommit(dir, "version.go", "chore: update version.go")
	if err != nil {
		return err
	}
	err = util.GitPush(dir, head)
	if err != nil {
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

func CreateGitHubRelease(ctx context.Context, version string) error {
	r, err := util.GetRelease(ctx, Owner, Repo, version)
	if err != nil {
		return err
	}
	if r != nil {
		fmt.Println("Release already exists")
		return nil
	}

	ki, err := GetIssue(ctx, version)
	if err != nil {
		return err
	}

	name := getGitHubReleaseName(version)
	body := getGitHubReleaseBody(version, ki.GetHTMLURL())
	prerelease, err := isGitHubReleasePrerelease(version)
	if err != nil {
		return err
	}

	r, err = util.CreateRelease(ctx, Owner, Repo, version, name, body, prerelease)
	if err != nil {
		return err
	}

	fmt.Printf("Release created: %s", r.GetHTMLURL())
	return nil
}
