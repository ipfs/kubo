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

type Main mg.Namespace

func getDevVersion(version string) (string, error) {
	v, err := sv.NewVersion(version)
	if err != nil {
		return "", err
	}
	nv := v.IncMinor()
	nv, err = nv.SetPrerelease("dev")
	if err != nil {
		return "", err
	}
	return nv.String(), nil
}

func getVersionUpdateBranchName(version string) string {
	return "version-update-" + semver.MajorMinor(version)
}

func getVersionUpdatePRTitle(version string) string {
	return "Update Version (dev): " + semver.MajorMinor(version)
}

func getVersionUpdatePRBody(version, url string) string {
	return fmt.Sprintf(
		`Related to %s

This PR updates the main dev version as part of the %s release.`,
		url, semver.MajorMinor(version))
}

func (Main) UpdateVersion(ctx context.Context, version string) error {
	v, err := getDevVersion(version)
	if err != nil {
		return err
	}

	f, err := util.GetFile(ctx, Owner, Repo, "version.go", DefaultBranchName)
	if err != nil {
		return err
	}

	if strings.Contains(*f.Content, v) {
		fmt.Println("Version already updated")
		return nil
	}

	head := getVersionUpdateBranchName(version)

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

	f, err = util.GetFile(ctx, Owner, Repo, "version.go", head)
	if err != nil {
		return err
	}

	if !strings.Contains(*f.Content, v) {
		dir, err := os.MkdirTemp("", "main")
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
		err = util.GitPushBranch(dir, head)
		if err != nil {
			return err
		}
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

	title := getVersionUpdatePRTitle(version)
	body := getVersionUpdatePRBody(version, ki.GetHTMLURL())

	pr, err = util.CreatePR(ctx, Owner, Repo, head, DefaultBranchName, title, body, false)
	if err != nil {
		return err
	}

	fmt.Printf("PR created: %s", pr.GetHTMLURL())
	return nil
}
