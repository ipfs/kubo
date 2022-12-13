package dist

import (
	"context"
	"fmt"
	"os"
	"strings"
	"net/http"
	"io"

	"dagger.io/dagger"
	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"github.com/ipfs/kubo/internal/mage/kubo"

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

	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer c.Close()

	container := c.Container().From("alpine:3.14.2")
	container = util.WithGit(container)
	container = util.WithBash(container)
	container = util.WithCheckout(container, Owner, Repo, branch.GetName(), branch.GetCommit().GetSHA())
	container = container.WithExec([]string{"./dist.sh", "add-version", "kubo", version})
	container = container.WithExec([]string{"git", "add", "dists/kubo/versions"})
	container = container.WithExec([]string{"git", "commit", "-m", "chore: release kubo " + version})
	container = container.WithExec([]string{"git", "push", "origin", head})

	stderr, err := container.Stderr(ctx)
	if err != nil {
		fmt.Println(stderr)
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
	// The next line performs the following curl command:
	// curl --retry 5 --no-progress-meter https://dist.ipfs.tech/kubo/versions | grep -q vX.Y.Z-rcN

	// Make a HTTP request to the dist.ipfs.tech/kubo/versions file
	r, err := http.Get("https://dist.ipfs.tech/kubo/versions")
	if err != nil {
		return err
	}
	versions, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if ! strings.Contains(string(versions), version) {
		return fmt.Errorf("Version %s not found in dist.ipfs.tech/kubo/versions", version)
	}

	fmt.Println("Version found in dist.ipfs.tech/kubo/versions")
	return nil
}
