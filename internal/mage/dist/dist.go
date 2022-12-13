package dist

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/ipfs/kubo/internal/mage/kubo"
	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
)

type Dist mg.Namespace

const (
	Owner             = "ipfs"
	Repo              = "distributions"
	DefaultBranchName = "master"
)

func getDistBranchName(version string) string {
	return "kubo-dist-" + version
}

func getDistPRTitle(version string) string {
	return "Kubo Dist: " + version
}

func getDistPRBody(version, url string) string {
	return fmt.Sprintf(
		`Related to %s

This PR is to release Kubo %s.`,
		url, version)
}

func (Dist) CreateDistPR(ctx context.Context, version string) error {
	versions, err := util.GetFile(ctx, Owner, Repo, "dists/kubo/versions", DefaultBranchName)
	if err != nil {
		return err
	}

	if strings.Contains(*versions.Content, version) {
		fmt.Println("Dist has already been released")
		return nil
	}

	head := getDistBranchName(version)

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

	ki, err := kubo.GetIssue(ctx, version)
	if err != nil {
		return err
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

	cmd := exec.Command("./dist.sh", "add-version", "kubo", version)
	cmd.Dir = dir
	err = cmd.Run()
	if err != nil {
		return err
	}

	err = util.GitCommit(dir, "dists/*/versions", "chore: release kubo "+version)
	if err != nil {
		return err
	}
	err = util.GitPush(dir, head)
	if err != nil {
		return err
	}

	title := getDistPRTitle(version)
	body := getDistPRBody(version, ki.GetHTMLURL())

	pr, err = util.CreatePR(ctx, Owner, Repo, head, DefaultBranchName, title, body, false)
	if err != nil {
		return err
	}

	fmt.Printf("PR created: %s", pr.GetHTMLURL())
	return nil
}

func (Dist) CheckIPFSTech(ctx context.Context, version string) error {
	r, err := http.Get("https://dist.ipfs.tech/kubo/versions")
	if err != nil {
		return err
	}
	versions, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if !strings.Contains(string(versions), version) {
		return fmt.Errorf("version %s not found in dist.ipfs.tech/kubo/versions", version)
	}

	fmt.Println("Version found in dist.ipfs.tech/kubo/versions")
	return nil
}
