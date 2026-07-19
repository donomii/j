package sandbox

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/donomii/j/types"
	"github.com/donomii/j/workspace"
)

type SandboxExecutor struct {
	workspaceRoot types.WorkspaceRoot
	useDocker     bool
	image         string
}

func New(workspaceRoot types.WorkspaceRoot, useDocker bool, image string) *SandboxExecutor {
	return &SandboxExecutor{workspaceRoot: workspaceRoot, useDocker: useDocker, image: image}
}

func (e *SandboxExecutor) Execute(command string, args []string, cwd string) (types.ExecutionResult, error) {
	if command == "" {
		return types.ExecutionResult{}, fmt.Errorf("execute command: executable is empty")
	}
	if !e.useDocker {
		return types.ExecutionResult{}, fmt.Errorf(
			"execute %q: container execution is disabled; set USE_DOCKER=true to confine commands to the approved workspace",
			command,
		)
	}
	return e.executeInDocker(command, args, cwd)
}

func (e *SandboxExecutor) executeInDocker(command string, args []string, cwd string) (types.ExecutionResult, error) {
	dockerArgs, err := e.containerArguments(command, args, cwd)
	if err != nil {
		return types.ExecutionResult{}, err
	}
	return e.run("docker", dockerArgs, "")
}

func (e *SandboxExecutor) containerArguments(command string, args []string, cwd string) ([]string, error) {
	requestedDirectory := cwd
	if requestedDirectory == "" {
		requestedDirectory = string(e.workspaceRoot)
	}
	resolvedDirectory, err := workspace.Resolve(e.workspaceRoot, requestedDirectory, true)
	if err != nil {
		return nil, fmt.Errorf("prepare container working directory %q: %w", requestedDirectory, err)
	}
	relativeDirectory, err := filepath.Rel(string(e.workspaceRoot), resolvedDirectory)
	if err != nil {
		return nil, fmt.Errorf("prepare container path for %q: %w", resolvedDirectory, err)
	}
	containerDirectory := filepath.ToSlash(filepath.Join("/workspace", relativeDirectory))
	dockerArgs := []string{
		"run", "--rm", "--network=none",
		"-v", string(e.workspaceRoot) + ":/workspace",
		"-w", containerDirectory,
		e.image,
		command,
	}
	return append(dockerArgs, args...), nil
}

func (e *SandboxExecutor) run(command string, args []string, cwd string) (types.ExecutionResult, error) {
	cmd := exec.Command(command, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			return types.ExecutionResult{}, fmt.Errorf("start executable %q with %d arguments: %w", command, len(args), err)
		}
	}
	return types.ExecutionResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}, nil
}
