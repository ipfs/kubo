package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ipfs/kubo/internal/mage/kubo"
	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"golang.org/x/mod/semver"
)

type Main mg.Namespace

const (
	Owner             = "ipfs"
	Repo              = "ipfs-desktop"
	DefaultBranchName = "main"
)

func getUpgradeBranchName(version string) string {
	return "kubo-upgrade-" + semver.MajorMinor(version)
}

func getUpgradePRTitle(version string) string {
	return "Kubo Upgrade: " + semver.MajorMinor(version)
}

func getUpgradePRBody(version, url string) string {
	return fmt.Sprintf(
		`Related to %s

This PR is to upgrade Kubo to %s.`,
		url, version)
}

func (Main) CreateOrUpdateUpgradePR(ctx context.Context, version string) error {
	versions, err := util.GetFile(ctx, Owner, Repo, "package.json", DefaultBranchName)
	if err != nil {
		return err
	}

	if strings.Contains(*versions.Content, version) {
		fmt.Println("Dist has already been released")
		return nil
	}

	head := getUpgradeBranchName(version)

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

	versions, err = util.GetFile(ctx, Owner, Repo, "package.json", head)
	if err != nil {
		return err
	}

	pr, err := util.GetPR(ctx, Owner, Repo, head)
	if err != nil {
		return err
	}

	if ! strings.Contains(*versions.Content, version) {
		dir, err := os.MkdirTemp("", "desktop")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)

		err = util.GitClone(dir, Owner, Repo, branch.GetName(), branch.GetCommit().GetSHA())
		if err != nil {
			return err
		}

		cmd := exec.Command("npm", "install", "go-ipfs@" + version, "--save")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println(string(out))
			return err
		}

		err = util.GitCommit(dir, "package*.json", "chore: upgrade go-ipfs "+version)
		if err != nil {
			return err
		}
		err = util.GitPushBranch(dir, head)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Branch has already been updated")
	}

	if pr == nil {
		ki, err := kubo.GetIssue(ctx, version)
		if err != nil {
			return err
		}

		title := getUpgradePRTitle(version)
		body := getUpgradePRBody(version, ki.GetHTMLURL())

		pr, err = util.CreatePR(ctx, Owner, Repo, head, DefaultBranchName, title, body, true)
		if err != nil {
			return err
		}

		fmt.Printf("PR created: %s", pr.GetHTMLURL())
	} else {
		fmt.Println("PR has already been created")
	}

	return nil
}

func (Main) CheckCI(ctx context.Context, version string) error {
	head := getUpgradeBranchName(version)

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
