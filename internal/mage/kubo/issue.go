package kubo

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v48/github"
	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/mod/semver"
)

type Issue mg.Namespace

func getIssueTitle(version string) string {
	return "Release " + strings.TrimLeft(semver.MajorMinor(version), "v")
}

func GetIssue(ctx context.Context, version string) (*github.Issue, error) {
	title := getIssueTitle(version)

	return util.GetIssue(ctx, Owner, Repo, title)
}

func getOutstandingTasks(markdown string) ([]string, error) {
	source := []byte(markdown)
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)

	node := md.Parser().Parse(text.NewReader(source)).FirstChild()

	for {
		if node == nil {
			return nil, fmt.Errorf("h2 not found")
		}
		if node.Kind() == ast.KindHeading {
			if node.(*ast.Heading).Level == 2 {
				if string(node.Text(source)) == "🗺 What's left for release" {
					break
				}
			}
		}
		node = node.NextSibling()
	}

	for {
		node = node.NextSibling()

		if node == nil {
			return nil, fmt.Errorf("h1 not found")
		}
		if node.Kind() == ast.KindHeading {
			if node.(*ast.Heading).Level < 3 {
				return nil, fmt.Errorf("h1 not found")
			}
			if node.(*ast.Heading).Level == 3 {
				if string(node.Text(source)) == "Required" {
					break
				}
			}
		}
	}

	for {
		node = node.NextSibling()

		if node == nil {
			return nil, fmt.Errorf("list not found")
		}
		if node.Kind() == ast.KindList {
			break
		}
	}

	var tasks []string
	err := ast.Walk(node, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if node.Kind() == east.KindTaskCheckBox && !node.(*east.TaskCheckBox).IsChecked {
				var content []string
				for {
					node = node.NextSibling()
					if node == nil {
						break
					}

					if node.Kind() == ast.KindText {
						content = append(content, string(node.Text(source)))
					} else if node.Kind() == ast.KindAutoLink {
						content = append(content, string(node.(*ast.AutoLink).URL(source)))
					} else {
						content = append(content, string("unknown content kind: "+node.Kind().String()))
					}
				}
				tasks = append(tasks, strings.Join(content, ""))
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

func (Issue) CheckOutstandingTasks(ctx context.Context, version string) error {
	i, err := GetIssue(ctx, version)
	if err != nil {
		return err
	}
	if i == nil {
		return fmt.Errorf("issue not found")
	}

	tasks, err := getOutstandingTasks(i.GetBody())
	if err != nil {
		return err
	}
	if len(tasks) != 0 {
		return fmt.Errorf("not all required tasks are complete: %#v", tasks)
	}
	return nil
}
