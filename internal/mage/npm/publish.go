package npm

import (
	"context"
	"fmt"
	"strings"

	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
)

type NPM mg.Namespace

const (
	Owner              = "ipfs"
	Repo               = "npm-go-ipfs"
	DefaultBranchName  = "master"
	PublishWorklowFile = "main.yml"
)

func (NPM) PublishToNPM(ctx context.Context, version string) error {
	run, err := util.GetWorkflowRun(ctx, Owner, Repo, PublishWorklowFile, true)
	if err != nil {
		return err
	}

	logs, err := util.GetWorkflowRunLogs(ctx, Owner, Repo, run.GetID())
	if err != nil {
		return err
	}

	publish := logs.JobLogs["publish"]
	if publish == nil {
		return fmt.Errorf("the latest run does not have a publish job")
	}

	if strings.Contains(publish.RawLogs, version) {
		fmt.Println("Version has already been published")
		return nil
	}

	return util.CreateWorkflowRun(ctx, Owner, Repo, PublishWorklowFile, DefaultBranchName)
}

func (NPM) CheckNPM(ctx context.Context, version string) error {
	run, err := util.GetWorkflowRun(ctx, Owner, Repo, PublishWorklowFile, true)
	if err != nil {
		return err
	}

	logs, err := util.GetWorkflowRunLogs(ctx, Owner, Repo, run.GetID())
	if err != nil {
		return err
	}

	publish := logs.JobLogs["publish"]
	if publish == nil {
		return fmt.Errorf("the latest run does not have a publish job")
	}

	if strings.Contains(publish.RawLogs, version) {
		fmt.Println("Version has already been published")
		return nil
	}

	return fmt.Errorf("version has not been published yet")
}
