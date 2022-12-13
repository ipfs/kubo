package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

func GitClone(path, owner, repo, branch, sha string) error {
	repository, err := git.PlainInit(path, false)
	if err != nil {
		return err
	}
	remote, err := repository.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/" + owner + "/" + repo},
	})
	if err != nil {
		return err
	}
	// https://github.com/go-git/go-git/issues/264
	err = remote.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("+" + sha + ":refs/remotes/origin/" + branch),
		},
		Tags:  git.NoTags,
		Depth: 1,
	})
	if err != nil {
		return err
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return err
	}
	return worktree.Checkout(&git.CheckoutOptions{
		Hash:   plumbing.NewHash("refs/remotes/origin/" + branch),
		Branch: plumbing.NewBranchReferenceName(branch),
		Create: true,
	})
}

func GitCommit(path, files, message string) error {
	repository, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return err
	}
	_, err = worktree.Add(files)
	if err != nil {
		return err
	}
	_, err = worktree.Commit(message, &git.CommitOptions{})
	return err
}

func GitPush(path, ref string) error {
	repository, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	return repository.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(ref),
		},
	})
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

func GetFile(ctx context.Context, owner, repo, path, ref string) (*github.RepositoryContent, error) {
	fmt.Printf("Getting file [owner: %s, repo: %s, path: %s, ref: %s]", owner, repo, path, ref)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	f, _, _, err := c.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: ref,
	})
	if err != nil && strings.Contains(err.Error(), "404 Not Found") {
		return nil, nil
	}
	return f, err
}

func GetCheckRuns(ctx context.Context, owner, repo, ref string) ([]*github.CheckRun, error) {
	fmt.Printf("Getting checks [owner: %s, repo: %s, ref: %s]", owner, repo, ref)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	opt := &github.ListCheckRunsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var runs []*github.CheckRun
	for {
		rs, r, err := c.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, opt)
		if err != nil {
			return nil, err
		}
		runs = append(runs, rs.CheckRuns...)
		if r.NextPage == 0 {
			break
		}
		opt.Page = r.NextPage
	}
	return runs, nil
}

func CreateWorkflowRun(ctx context.Context, owner, repo, file, ref string) error {
	fmt.Printf("Creating workflow run [owner: %s, repo: %s, file: %s, ref: %s]", owner, repo, file, ref)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return err
	}

	_, err = c.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, file, github.CreateWorkflowDispatchEventRequest{
		Ref: ref,
	})
	return err
}

func GetWorkflowRun(ctx context.Context, owner, repo, file string, completed bool) (*github.WorkflowRun, error) {
	fmt.Printf("Getting workflow run [owner: %s, repo: %s, file: %s, completed: %v]", owner, repo, file, completed)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	opt := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	}
	if completed {
		opt.Status = "completed"
	}
	r, _, err := c.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, file, opt)
	if err != nil {
		return nil, err
	}
	if len(r.WorkflowRuns) == 0 {
		return nil, nil
	}
	return r.WorkflowRuns[0], nil
}

func GetWorkflowRunLogs(ctx context.Context, owner, repo string, id int64) (string, error) {
	fmt.Printf("Getting workflow run logs [owner: %s, repo: %s, id: %v]", owner, repo, id)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return "", err
	}

	url, _, err := c.Actions.GetWorkflowRunLogs(ctx, owner, repo, id, true)
	if err != nil {
		return "", err
	}

	r, err := http.Get(url.String())
	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(r.Body)
	return string(b), err
}

func GetRelease(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, error) {
	fmt.Printf("Getting release [owner: %s, repo: %s, tag: %s]", owner, repo, tag)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	r, _, err := c.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil && strings.Contains(err.Error(), "404 Not Found") {
		return nil, nil
	}
	return r, err
}

func CreateRelease(ctx context.Context, owner, repo, tag, name, body string, prerelease bool) (*github.RepositoryRelease, error) {
	fmt.Printf("Creating release [owner: %s, repo: %s, tag: %s]", owner, repo, tag)
	fmt.Println()

	c, err := GitHubClient()
	if err != nil {
		return nil, err
	}

	r, _, err := c.Repositories.CreateRelease(ctx, owner, repo, &github.RepositoryRelease{
		TagName:    &tag,
		Name:       &name,
		Body:       &body,
		Prerelease: &prerelease,
	})
	return r, err
}
