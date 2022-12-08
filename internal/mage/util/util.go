package util

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"

	"dagger.io/dagger"
)

func WithGitSecrets(client *dagger.Client, container *dagger.Container) *dagger.Container {
	secret := client.Host().EnvVariable("GITHUB_TOKEN").Secret()

	return container.
		WithSecretVariable("GITHUB_TOKEN", secret)
}

func WithGit(container *dagger.Container) *dagger.Container {
	token := os.Getenv("GITHUB_TOKEN")
	// TODO: This should really be a secret
	auth := base64.URLEncoding.EncodeToString([]byte("pat:" + token))

	name := os.Getenv("GITHUB_USER_NAME")
	if name == "" {
		name = "Kubo Mage"
	}
	email := os.Getenv("GITHUB_USER_EMAIL")
	if email == "" {
		email = "noreply+kubo-mage@ipfs.tech"
	}

	return container.
		WithExec([]string{"apk", "add", "git"}).
		WithExec([]string{"git", "config", "--global", "gc.auto", "0"}).
		WithExec([]string{"git", "config", "--global", "core.sshCommand", ""}).
		WithExec([]string{"git", "config", "--global", "http.https://github.com/.extraheader", "AUTHORIZATION: basic " + auth}).
		WithExec([]string{"git", "config", "--global", "user.name", name}).
		WithExec([]string{"git", "config", "--global", "user.email", email})
}

func WithCheckout(container *dagger.Container, owner, repo, branch, sha string) *dagger.Container {
	return container.
		WithExec([]string{"git", "init"}).
		WithExec([]string{"git", "remote", "add", "origin", "https://github.com/" + owner + "/" + repo}).
		WithExec([]string{"git", "-c", "protocol.version=2", "fetch", "--no-tags", "--prune", "--no-recurse-submodules", "--depth=1", "origin", "+" + sha + ":refs/remotes/origin/" + branch}).
		WithExec([]string{"git", "checkout", "--force", "-B", branch, "refs/remotes/origin/" + branch})
}

func GitHubClient() (*github.Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("env var GITHUB_TOKEN must be set")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc), nil
}

func GetIssue(ctx context.Context, owner, repo, title string) (*github.Issue, error) {
	fmt.Printf("Getting issue [owner: %s, repo: %s, title: %s]", owner, repo, title)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	opt := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	q := fmt.Sprintf("is:issue repo:%s/%s in:title %s", owner, repo, title)
	var issue *github.Issue
	for {
		is, r, err := c.Search.Issues(ctx, q, opt)
		if err != nil {
			return nil, err
		}
		for _, i := range is.Issues {
			if i.GetTitle() == title {
				issue = i
				break
			}
		}
		if issue != nil || r.NextPage == 0 {
			break
		}
		opt.Page = r.NextPage
	}

	return issue, nil
}

func CreateIssue(ctx context.Context, owner, repo, title, body string) (*github.Issue, error) {
	fmt.Printf("Creating issue [owner: %s, repo: %s, title: %s]", owner, repo, title)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	issue, _, err := c.Issues.Create(ctx, owner, repo, &github.IssueRequest{
		Title: &title,
		Body:  &body,
	})
	return issue, err
}

func GetIssueComment(ctx context.Context, owner, repo, title, body string) (*github.IssueComment, error) {
	fmt.Printf("Getting issue comment [owner: %s, repo: %s, title: %s, body: %s]", owner, repo, title, body)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	issue, err := GetIssue(ctx, owner, repo, title)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, nil
	}

	opt := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var comment *github.IssueComment
	for {
		cs, r, err := c.Issues.ListComments(ctx, owner, repo, issue.GetNumber(), opt)
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			if c.GetBody() == body {
				comment = c
				break
			}
		}
		if comment != nil || r.NextPage == 0 {
			break
		}
		opt.Page = r.NextPage
	}

	return comment, nil
}

func CreateIssueComment(ctx context.Context, owner, repo, title, body string) (*github.IssueComment, error) {
	fmt.Printf("Creating issue comment [owner: %s, repo: %s, title: %s, body: %s]", owner, repo, title, body)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	issue, err := GetIssue(ctx, owner, repo, title)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}

	comment, _, err := c.Issues.CreateComment(ctx, owner, repo, issue.GetNumber(), &github.IssueComment{
		Body: &body,
	})
	return comment, err
}

func GetBranch(ctx context.Context, owner, repo, name string) (*github.Branch, error) {
	fmt.Printf("Getting branch [owner: %s, repo: %s, name: %s]", owner, repo, name)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	branch, _, err := c.Repositories.GetBranch(ctx, owner, repo, name, false)
	if err != nil && strings.Contains(err.Error(), "404 Not Found") {
		return nil, nil
	}
	return branch, err
}

func CreateBranch(ctx context.Context, owner, repo, name, source string) (*github.Branch, error) {
	fmt.Printf("Creating branch [owner: %s, repo: %s, name: %s, source: %s]", owner, repo, name, source)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	r, _, err := c.Git.GetRef(ctx, owner, repo, "refs/heads/"+source)
	if err != nil {
		return nil, err
	}

	_, _, err = c.Git.CreateRef(ctx, owner, repo, &github.Reference{
		Ref:    github.String("refs/heads/" + name),
		Object: r.GetObject(),
	})
	if err != nil {
		return nil, err
	}

	return GetBranch(ctx, owner, repo, name)
}

func GetPR(ctx context.Context, owner, repo, head string) (*github.PullRequest, error) {
	fmt.Printf("Getting PR [owner: %s, repo: %s, head: %s]", owner, repo, head)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	q := fmt.Sprintf("is:pr repo:%s/%s head:%s", owner, repo, head)
	r, _, err := c.Search.Issues(ctx, q, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if err != nil {
		return nil, err
	}
	if len(r.Issues) == 0 {
		return nil, nil
	}

	n := r.Issues[0].GetNumber()

	pr, _, err := c.PullRequests.Get(ctx, owner, repo, n)
	return pr, err
}

func CreatePR(ctx context.Context, owner, repo, head, base, title, body string, draft bool) (*github.PullRequest, error) {
	fmt.Printf("Creating PR [owner: %s, repo: %s, head: %s, base: %s, title: %s, body: %s, draft: %t]", owner, repo, head, base, title, body, draft)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	pr, _, err := c.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: &title,
		Head:  &head,
		Base:  &base,
		Body:  &body,
		Draft: &draft,
	})
	return pr, err
}
