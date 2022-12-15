package kubo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"golang.org/x/mod/semver"
)

type Changelog mg.Namespace

func getChangelogBranchName(version string) string {
	return "changelog-" + version
}

func getChangelogPRTitle(version string) string {
	return "Changelog: " + version
}

func getChangelogPRBody(version, url string) string {
	return fmt.Sprintf(
		`Related to %s

This PR is to add and link the changelog ahead of the %s release.`,
		url, version)
}

func (Changelog) CreateChangelog(ctx context.Context, version string) error {
	v := semver.MajorMinor(version)
	l := fmt.Sprintf("[%s](docs/changelogs/%s.md)", v, v)

	changelog, err := util.GetFile(ctx, Owner, Repo, "docs/changelogs/"+v+".md", DefaultBranchName)
	if err != nil {
		return err
	}

	list, err := util.GetFile(ctx, Owner, Repo, "CHANGELOG.md", DefaultBranchName)
	if err != nil {
		return err
	}

	if changelog != nil && strings.Contains(*list.Content, l) {
		fmt.Printf("Changelog already exists: %s", changelog.GetHTMLURL())
		fmt.Printf("Changelog is already listed in CHANGELOG.md")
		return nil
	}

	head := getChangelogBranchName(version)

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

	dir, err := os.MkdirTemp("", "changelog")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	err = util.GitClone(dir, Owner, Repo, branch.GetName(), branch.GetCommit().GetSHA())
	if err != nil {
		return err
	}

	if changelog == nil {
		cmd := exec.Command("echo", "", ">", "docs/changelogs/"+v+".md")
		cmd.Dir = dir
		err = cmd.Run()
		if err != nil {
			return err
		}
		err = util.GitCommit(dir, "docs/changelogs/"+v+".md", "docs: add changelog")
		if err != nil {
			return err
		}
	}
	if !strings.Contains(*list.Content, l) {
		cmd := exec.Command("sed", "-i", "3i - "+l, "CHANGELOG.md")
		cmd.Dir = dir
		err = cmd.Run()
		if err != nil {
			return err
		}
		err = util.GitCommit(dir, "CHANGELOG.md", "docs: update CHANGELOG.md")
		if err != nil {
			return err
		}
	}

	err = util.GitPushBranch(dir, head)
	if err != nil {
		return err
	}

	title := getChangelogPRTitle(version)
	body := getChangelogPRBody(version, ki.GetHTMLURL())

	pr, err = util.CreatePR(ctx, Owner, Repo, head, DefaultBranchName, title, body, true)
	if err != nil {
		return err
	}

	fmt.Printf("PR created: %s", pr.GetHTMLURL())
	return nil
}
