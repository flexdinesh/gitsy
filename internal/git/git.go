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
	return RunContext(context.Background(), cwd, args...)
}

func RunContext(ctx context.Context, cwd string, args ...string) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	fullArgs := append([]string{"-C", cwd}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	return runCommand(ctx, cmd)
}

func runCommand(ctx context.Context, cmd *exec.Cmd) Result {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	status := 0
	if err != nil {
		status = -1
		if ctx.Err() != nil {
			stderr.WriteString(ctx.Err().Error())
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

func TopLevel(repoPath string) Result {
	return TopLevelContext(context.Background(), repoPath)
}

func TopLevelContext(ctx context.Context, repoPath string) Result {
	return RunContext(ctx, repoPath, "rev-parse", "--show-toplevel")
}

func ShortStatus(repoPath string) Result {
	return ShortStatusContext(context.Background(), repoPath)
}

func ShortStatusContext(ctx context.Context, repoPath string) Result {
	return RunContext(ctx, repoPath, "status", "--short", "--branch", "--ahead-behind")
}

func WorktreeList(repoPath string) Result {
	return WorktreeListContext(context.Background(), repoPath)
}

func WorktreeListContext(ctx context.Context, repoPath string) Result {
	return RunContext(ctx, repoPath, "worktree", "list", "--porcelain")
}

func FastForward(repoPath string) Result {
	return FastForwardContext(context.Background(), repoPath)
}

func FastForwardContext(ctx context.Context, repoPath string) Result {
	return RunContext(ctx, repoPath, "merge", "--ff-only")
}

func FetchAll(repoPath string, timeout time.Duration) Result {
	return FetchAllContext(context.Background(), repoPath, timeout)
}

func FetchAllContext(ctx context.Context, repoPath string, timeout time.Duration) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "fetch", "--all")
	result := runCommand(ctx, cmd)
	if ctx.Err() == context.DeadlineExceeded {
		result.Stderr = "git fetch timed out"
	}
	return result
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
