package companion

import (
	"context"
	"fmt"
	"strings"

	"github.com/ipfs/kubo/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
)

type Test mg.Namespace

const (
	Owner             = "ipfs"
	Repo              = "ipfs-companion"
	DefaultBranchName = "main"
	TestWorklowFile   = "e2e.yml"
)

func (Test) InitTest(ctx context.Context, version string) error {
	return util.CreateWorkflowRun(ctx, Owner, Repo, TestWorklowFile, DefaultBranchName, util.WorkflowRunInput{Name: "kubo-version", Value: version})
}

func (Test) CheckTest(ctx context.Context, version string) error {
	run, err := util.GetWorkflowRun(ctx, Owner, Repo, TestWorklowFile, false)
	if err != nil {
		return err
	}

	if (run.GetStatus() != "completed") {
		return fmt.Errorf("the latest run is not completed yet")
	}

	logs, err := util.GetWorkflowRunLogs(ctx, Owner, Repo, run.GetID())
	if err != nil {
		return err
	}

	test := logs.JobLogs["test"]
	if test == nil {
		return fmt.Errorf("the latest run does not have a test job")
	}

	if ! strings.Contains(test.RawLogs, fmt.Sprintf("KUBO_VERSION: %s", version)) {
		return fmt.Errorf("the latest run is not for version %s", version)
	}

	if run.GetConclusion() == "success" {
		fmt.Println("The latest run succeeded")
		return nil
	}
	return fmt.Errorf("the latest run did not succeed")
}
