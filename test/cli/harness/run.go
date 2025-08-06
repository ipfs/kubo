package harness

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Runner is a process runner which can run subprocesses and aggregate output.
type Runner struct {
	Env     map[string]string
	Dir     string
	Verbose bool
}

type (
	CmdOpt  func(*exec.Cmd)
	RunFunc func(*exec.Cmd) error
)

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
	Stdout  *Buffer
	Stderr  *Buffer
	Err     error
	ExitErr *exec.ExitError
	Cmd     *exec.Cmd
}

func (r *RunResult) ExitCode() int {
	return r.Cmd.ProcessState.ExitCode()
}

func environToMap(environ []string) map[string]string {
	m := map[string]string{}
	for _, e := range environ {
		kv := strings.Split(e, "=")
		// Skip environment variables that start with =
		// These can occur in Windows https://github.com/golang/go/issues/61956
		if kv[0] == "" {
			continue
		}
		m[kv[0]] = kv[1]
	}
	return m
}

func (r *Runner) Run(req RunRequest) *RunResult {
	cmd := exec.Command(req.Path, req.Args...)
	var stdout io.Writer
	var stderr io.Writer
	outbuf := &Buffer{}
	errbuf := &Buffer{}

	if r.Verbose {
		or, ow := io.Pipe()
		errr, errw := io.Pipe()
		stdout = io.MultiWriter(outbuf, ow)
		stderr = io.MultiWriter(errbuf, errw)
		go func() {
			_, _ = io.Copy(os.Stdout, or)
		}()
		go func() {
			_, _ = io.Copy(os.Stderr, errr)
		}()
	} else {
		stdout = outbuf
		stderr = errbuf
	}

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

	log.Debugf("running %v", cmd.Args)

	err := req.RunFunc(cmd)

	result := RunResult{
		Stdout: outbuf,
		Stderr: errbuf,
		Cmd:    cmd,
		Err:    err,
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitErr = exitErr
	}

	return &result
}

// MustRun runs the command and fails the test if the command fails.
func (r *Runner) MustRun(req RunRequest) *RunResult {
	result := r.Run(req)
	r.AssertNoError(result)
	return result
}

func (r *Runner) AssertNoError(result *RunResult) {
	if result.ExitErr != nil {
		log.Panicf("'%s' returned error, code: %d, err: %s\nstdout:%s\nstderr:%s\n",
			result.Cmd.Args, result.ExitErr.ExitCode(), result.ExitErr.Error(), result.Stdout.String(), result.Stderr.String())
	}
	if result.Err != nil {
		log.Panicf("unable to run %s: %s", result.Cmd.Path, result.Err)
	}
}

func RunWithEnv(env map[string]string) CmdOpt {
	return func(cmd *exec.Cmd) {
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
}

func RunWithPath(path string) CmdOpt {
	return func(cmd *exec.Cmd) {
		var newEnv []string
		for _, env := range cmd.Env {
			e := strings.Split(env, "=")
			if e[0] == "PATH" {
				paths := strings.Split(e[1], ":")
				paths = append(paths, path)
				e[1] = strings.Join(paths, ":")
				fmt.Printf("path: %s\n", strings.Join(e, "="))
			}
			newEnv = append(newEnv, strings.Join(e, "="))
		}
		cmd.Env = newEnv
	}
}

func RunWithStdin(reader io.Reader) CmdOpt {
	return func(cmd *exec.Cmd) {
		cmd.Stdin = reader
	}
}

func RunWithStdinStr(s string) CmdOpt {
	return RunWithStdin(strings.NewReader(s))
}

func RunWithStdout(writer io.Writer) CmdOpt {
	return func(cmd *exec.Cmd) {
		cmd.Stdout = io.MultiWriter(writer, cmd.Stdout)
	}
}

func RunWithStderr(writer io.Writer) CmdOpt {
	return func(cmd *exec.Cmd) {
		cmd.Stderr = io.MultiWriter(writer, cmd.Stdout)
	}
}
