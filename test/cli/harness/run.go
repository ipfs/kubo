package harness

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
)

type Runner struct {
	Env     map[string]string
	Dir     string
	Verbose bool
}

type CmdOpt func(*exec.Cmd)
type RunFunc func(*exec.Cmd) error

var RunFuncStart = (*exec.Cmd).Start

type RunRequest struct {
	Path string
	Args []string
	// Options that are applied to the exec.Cmd just before running it
	CmdOpts []CmdOpt
	// Function to use to run the command.
	// If not specified, defaults to cmd.Run
	RunFunc func(*exec.Cmd) error
	Verbose bool
}

type RunResult struct {
	Stdout  *bytes.Buffer
	Stderr  *bytes.Buffer
	Err     error
	ExitErr *exec.ExitError
	Cmd     *exec.Cmd
}

func (r *Runner) Run(req RunRequest) RunResult {
	cmd := exec.Command(req.Path, req.Args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = r.Dir

	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	for _, o := range req.CmdOpts {
		o(cmd)
	}

	if req.RunFunc == nil {
		req.RunFunc = (*exec.Cmd).Run
	}

	log.Printf("running %v", cmd.Args)

	err := req.RunFunc(cmd)

	result := RunResult{
		Stdout: stdout,
		Stderr: stderr,
		Cmd:    cmd,
		Err:    err,
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitErr = exitErr
	}

	return result
}

// MustRun runs the command and fails the test if the command fails.
func (r *Runner) MustRun(req RunRequest) RunResult {
	result := r.Run(req)
	r.AssertNoError(result)
	return result
}

func (r *Runner) AssertNoError(result RunResult) {
	if result.ExitErr != nil {
		log.Fatalf("'%s' returned error, code: %d, err: %s\nstdout:%s\nstderr:%s\n",
			result.Cmd.Args, result.ExitErr.ExitCode(), result.ExitErr.Error(), result.Stdout.String(), result.Stderr.String())

	}
	if result.Err != nil {
		log.Fatalf("unable to run %s: %s", result.Cmd.Path, result.Err)

	}
}

func (r *Runner) RunWithEnv(env map[string]string) CmdOpt {
	return func(cmd *exec.Cmd) {
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
}
