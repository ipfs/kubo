package util

import (
	"context"
	"fmt"
	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"
	"os"

	"dagger.io/dagger"
)

func alpineBase(i string, c *dagger.Client) *dagger.Container {
	return c.Container().
		From(i).
		WithExec([]string{"apk", "add", "build-base"}).
		WithExec([]string{"apk", "add", "git"}).
		WithWorkdir("/app")
}

func NodeBase(c *dagger.Client) *dagger.Container {
	return alpineBase("node:16-alpine", c)
}

func GoBase(c *dagger.Client) *dagger.Container {
	return alpineBase("golang:1.19-alpine", c)
}

func WithSetHostVar(ctx context.Context, h *dagger.Host, varName string) *dagger.HostVariable {
	hv := h.EnvVariable(varName)
	if val, _ := hv.Secret().Plaintext(ctx); val == "" {
		fmt.Fprintf(os.Stderr, "env var %s must be set", varName)
		os.Exit(1)
	}
	return hv
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
	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	branch, _, err := c.Repositories.GetBranch(ctx, owner, repo, name, false)
	return branch, err
}

func CreateBranch(ctx context.Context, owner, repo, name, sha string) (*github.Branch, error) {
	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	_, _, err = c.Git.CreateRef(ctx, owner, repo, &github.Reference{
		Ref: github.String("refs/heads/" + name),
		Object: &github.GitObject{
			SHA: github.String(sha),
		},
	})
	if err != nil {
		return nil, err
	}

	return GetBranch(ctx, owner, repo, name)
}

func GetPR(ctx context.Context, owner, repo, head string) (*github.PullRequest, error) {
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
