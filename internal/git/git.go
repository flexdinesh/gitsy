package git

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	OK     bool
	Stdout string
	Stderr string
	Status int
}

func Run(cwd string, args ...string) Result {
	fullArgs := append([]string{"-C", cwd}, args...)
	cmd := exec.Command("git", fullArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	status := 0
	if err != nil {
		status = -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			status = exitErr.ExitCode()
		} else if stderr.Len() == 0 {
			stderr.WriteString(err.Error())
		}
	}

	return Result{
		OK:     err == nil,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Status: status,
	}
}

func TopLevel(repoPath string) Result {
	return Run(repoPath, "rev-parse", "--show-toplevel")
}

func ShortStatus(repoPath string) Result {
	return Run(repoPath, "status", "--short", "--branch", "--ahead-behind")
}

func WorktreeList(repoPath string) Result {
	return Run(repoPath, "worktree", "list", "--porcelain")
}

func FastForward(repoPath string) Result {
	return Run(repoPath, "merge", "--ff-only")
}

func FetchAll(repoPath string, timeout time.Duration) Result {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "fetch", "--all")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	status := 0
	if err != nil {
		status = -1
		if ctx.Err() == context.DeadlineExceeded {
			stderr.WriteString("git fetch timed out")
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			status = exitErr.ExitCode()
		} else if stderr.Len() == 0 {
			stderr.WriteString(err.Error())
		}
	}

	return Result{
		OK:     err == nil,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Status: status,
	}
}

func ParseWorktreePaths(porcelain string) []string {
	paths := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(porcelain, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	return paths
}
